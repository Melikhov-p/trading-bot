package pipeline

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"trading-bot/internal/adapters/broker/noop"
	"trading-bot/internal/adapters/decision/weighted"
	"trading-bot/internal/adapters/filter/rulebased"
	"trading-bot/internal/adapters/newsprovider/filesource"
	"trading-bot/internal/adapters/repository/memory"
	"trading-bot/internal/adapters/sizing/fixed"
	"trading-bot/internal/application/config"
	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

var fixedNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

// fakeAnalyzer — тестовый port.NewsAnalyzer, отдающий заранее заданный
// NewsAnalysis по News.ID, без обращения к реальной LLM.
type fakeAnalyzer struct {
	byNewsID map[string]news.NewsAnalysis
}

func (f *fakeAnalyzer) Name() string { return "fake" }

func (f *fakeAnalyzer) Analyze(_ context.Context, n news.News) (news.NewsAnalysis, error) {
	a, ok := f.byNewsID[n.ID]
	if !ok {
		return news.NewsAnalysis{}, fmt.Errorf("fakeAnalyzer: no analysis for news id %q", n.ID)
	}
	return a, nil
}

var _ port.NewsAnalyzer = (*fakeAnalyzer)(nil)

// countingOrderRepo оборачивает memory.OrderRepository, считая вызовы Save —
// нужно проверить идемпотентность исполнения без добавления List/FindByID
// в продовый port.OrderRepository.
type countingOrderRepo struct {
	*memory.OrderRepository
	mu        sync.Mutex
	saveCount int
}

func newCountingOrderRepo() *countingOrderRepo {
	return &countingOrderRepo{OrderRepository: memory.NewOrderRepository()}
}

func (r *countingOrderRepo) Save(ctx context.Context, o trading.Order) error {
	r.mu.Lock()
	r.saveCount++
	r.mu.Unlock()
	return r.OrderRepository.Save(ctx, o)
}

func testConfig() config.OrchestratorConfig {
	return config.OrchestratorConfig{
		Version:                "test",
		WorkerPoolSize:         2,
		FixedPositionQuantity:  5,
		Weights:                config.Weights{Sentiment: 0.6, Surprise: 0.4},
		UnknownSurprisePenalty: 0,
		MixedSentimentPenalty:  0.5,
		EventModifiers:         map[string]float64{"earnings": 1.0},
		Thresholds: map[string]config.ThresholdSet{
			"intraday": {
				MinConfidence:    0.5,
				MinImpact:        0.5,
				BuyThreshold:     0.3,
				SellThreshold:    0.3,
				WatchThreshold:   0.2,
				SignalTTLSeconds: 3600,
				CooldownSeconds:  1800,
			},
		},
		Filter: config.FilterConfig{
			MinBodyLength:      20,
			MaxAgeSeconds:      864000,
			DedupWindowSeconds: 3600,
			WatchlistWeight:    0.6,
			KeywordWeight:      0.4,
			MinScoreToPass:     0.5,
		},
		Watchlist: []string{"SBER"},
		Keywords:  map[string][]string{"earnings": {"прибыль"}},
	}
}

func buildTestOrchestrator(cfg config.OrchestratorConfig, analyzer port.NewsAnalyzer, orderRepo *countingOrderRepo, newsPath string) *Orchestrator {
	clk := fakeClock{now: fixedNow}

	filterCfg := rulebased.Config{
		MinBodyLength:   cfg.Filter.MinBodyLength,
		MaxAge:          cfg.Filter.MaxAge(),
		DedupWindow:     cfg.Filter.DedupWindow(),
		WatchlistWeight: cfg.Filter.WatchlistWeight,
		KeywordWeight:   cfg.Filter.KeywordWeight,
		MinScoreToPass:  cfg.Filter.MinScoreToPass,
		Watchlist:       cfg.Watchlist,
		Keywords:        cfg.Keywords,
	}

	return New(Deps{
		Provider:     filesource.New(newsPath),
		Filter:       rulebased.New(filterCfg, clk),
		Analyzer:     analyzer,
		Engine:       weighted.New(cfg, clk),
		Sizer:        fixed.New(cfg.FixedPositionQuantity),
		Broker:       noop.New(slog.New(slog.NewTextHandler(io.Discard, nil))),
		NewsRepo:     memory.NewNewsRepository(),
		AnalysisRepo: memory.NewAnalysisRepository(),
		SignalRepo:   memory.NewSignalRepository(clk),
		OrderRepo:    orderRepo,
		Config:       cfg,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func strongAnalysis() news.NewsAnalysis {
	return news.NewsAnalysis{
		NewsID:               "n-strong",
		Company:              "Сбербанк",
		Ticker:               tickerPtr("SBER"),
		EventType:            news.EventTypeEarnings,
		FundamentalSentiment: news.SentimentPositive,
		Surprise:             news.SurprisePositive,
		ImpactScore:          mustScore(0.9),
		Confidence:           mustScore(0.9),
		TimeHorizon:          news.HorizonIntraday,
	}
}

func weakAnalysis() news.NewsAnalysis {
	return news.NewsAnalysis{
		NewsID:               "n-weak",
		Company:              "Сбербанк",
		Ticker:               tickerPtr("SBER"),
		EventType:            news.EventTypeEarnings,
		FundamentalSentiment: news.SentimentPositive,
		Surprise:             news.SurprisePositive,
		ImpactScore:          mustScore(0.9),
		Confidence:           mustScore(0.3), // ниже MinConfidence => Action=Hold
		TimeHorizon:          news.HorizonIntraday,
	}
}

func tickerPtr(t trading.Ticker) *trading.Ticker { return &t }

func mustScore(v float64) news.Score {
	s, err := news.NewScore(v)
	if err != nil {
		panic(err)
	}
	return s
}

func TestOrchestrator_Run_ProcessesFixture(t *testing.T) {
	cfg := testConfig()
	analyzer := &fakeAnalyzer{byNewsID: map[string]news.NewsAnalysis{
		"n-strong": strongAnalysis(),
		"n-weak":   weakAnalysis(),
	}}
	orderRepo := newCountingOrderRepo()
	orch := buildTestOrchestrator(cfg, analyzer, orderRepo, "testdata/news.ndjson")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if orderRepo.saveCount != 1 {
		t.Fatalf("orderRepo.saveCount = %d, want 1", orderRepo.saveCount)
	}

	strongNews, found, err := orch.newsRepo.FindByExternalID(ctx, "test", "ext-strong")
	if err != nil || !found {
		t.Fatalf("find n-strong: found=%v err=%v", found, err)
	}
	if strongNews.Status != news.StatusExecuted {
		t.Fatalf("n-strong status = %q, want %q", strongNews.Status, news.StatusExecuted)
	}

	junkNews, found, err := orch.newsRepo.FindByExternalID(ctx, "test", "ext-junk")
	if err != nil || !found {
		t.Fatalf("find n-junk: found=%v err=%v", found, err)
	}
	if junkNews.Status != news.StatusFilteredOut {
		t.Fatalf("n-junk status = %q, want %q", junkNews.Status, news.StatusFilteredOut)
	}

	weakNews, found, err := orch.newsRepo.FindByExternalID(ctx, "test", "ext-weak")
	if err != nil || !found {
		t.Fatalf("find n-weak: found=%v err=%v", found, err)
	}
	if weakNews.Status != news.StatusAnalyzed {
		t.Fatalf("n-weak status = %q, want %q (signal skipped, news stays analyzed)", weakNews.Status, news.StatusAnalyzed)
	}
}

func TestOrchestrator_ProcessNews_DedupIsIdempotent(t *testing.T) {
	cfg := testConfig()
	analyzer := &fakeAnalyzer{byNewsID: map[string]news.NewsAnalysis{
		"n-strong": strongAnalysis(),
	}}
	orderRepo := newCountingOrderRepo()
	orch := buildTestOrchestrator(cfg, analyzer, orderRepo, "testdata/news.ndjson")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	n := news.News{
		ID:          "n-strong",
		Source:      "test",
		ExternalID:  "ext-strong",
		Headline:    "СБербанк нарастил прибыль",
		Body:        "Сбербанк отчитался о росте прибыли по итогам квартала выше ожиданий рынка.",
		PublishedAt: time.Date(2026, 9, 11, 11, 0, 0, 0, time.UTC),
		Tickers:     []trading.Ticker{"SBER"},
	}

	if err := orch.ProcessNews(ctx, n); err != nil {
		t.Fatalf("ProcessNews (1st call): %v", err)
	}
	if err := orch.ProcessNews(ctx, n); err != nil {
		t.Fatalf("ProcessNews (2nd call): %v", err)
	}

	if orderRepo.saveCount != 1 {
		t.Fatalf("orderRepo.saveCount = %d, want 1 (second call must be a dedup no-op)", orderRepo.saveCount)
	}
}
