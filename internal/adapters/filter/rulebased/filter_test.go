package rulebased

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func baseConfig() Config {
	return Config{
		MinBodyLength:   10,
		MaxAge:          24 * time.Hour,
		DedupWindow:     time.Hour,
		WatchlistWeight: 0.6,
		KeywordWeight:   0.4,
		MinScoreToPass:  0.5,
		Watchlist:       []string{"SBER"},
		Keywords: map[string][]string{
			"earnings": {"прибыль", "выручка"},
		},
	}
}

func baseNews() news.News {
	return news.News{
		Source:      "test",
		Headline:    "Заголовок новости",
		Body:        "Достаточно длинный текст новости без ключевых слов и тикеров.",
		PublishedAt: fixedNow,
	}
}

var fixedNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func TestFilter_IsRelevant(t *testing.T) {
	tests := []struct {
		name          string
		cfg           Config
		news          news.News
		wantRelevant  bool
		wantReason    string
		wantHint      news.EventType
		wantScoreMore float64 // score >= wantScoreMore, если > 0
	}{
		{
			name: "body too short",
			cfg:  baseConfig(),
			news: func() news.News {
				n := baseNews()
				n.Body = "мало"
				return n
			}(),
			wantRelevant: false,
			wantReason:   "body too short",
		},
		{
			name: "news too old",
			cfg:  baseConfig(),
			news: func() news.News {
				n := baseNews()
				n.PublishedAt = fixedNow.Add(-48 * time.Hour)
				return n
			}(),
			wantRelevant: false,
			wantReason:   "news too old",
		},
		{
			name:         "no watchlist and no keyword hit",
			cfg:          baseConfig(),
			news:         baseNews(),
			wantRelevant: false,
			wantReason:   "score below threshold",
		},
		{
			name: "watchlist hit reaches threshold",
			cfg:  baseConfig(),
			news: func() news.News {
				n := baseNews()
				n.Tickers = []trading.Ticker{"SBER"}
				return n
			}(),
			wantRelevant:  true,
			wantScoreMore: 0.6,
		},
		{
			name: "keyword hit alone is below threshold",
			cfg:  baseConfig(),
			news: func() news.News {
				n := baseNews()
				n.Body = "Компания отчиталась про рекордную чистую прибыль в этом квартале."
				return n
			}(),
			wantRelevant: false,
			wantReason:   "score below threshold",
			wantHint:     news.EventTypeEarnings,
		},
		{
			name: "watchlist and keyword force-pass despite unreachable threshold",
			cfg: func() Config {
				cfg := baseConfig()
				cfg.MinScoreToPass = 1.5 // недостижимо суммой весов 0.6+0.4
				return cfg
			}(),
			news: func() news.News {
				n := baseNews()
				n.Tickers = []trading.Ticker{"SBER"}
				n.Body = "Компания отчиталась про рекордную чистую прибыль в этом квартале."
				return n
			}(),
			wantRelevant: true,
			wantHint:     news.EventTypeEarnings,
		},
		{
			name: "watchlist hit by company name is case-insensitive",
			cfg:  baseConfig(),
			news: func() news.News {
				n := baseNews()
				n.Companies = []string{"sber"}
				return n
			}(),
			wantRelevant:  true,
			wantScoreMore: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.cfg, &fakeClock{now: fixedNow})

			got, err := f.IsRelevant(context.Background(), tt.news)
			if err != nil {
				t.Fatalf("IsRelevant() error = %v", err)
			}

			if got.Relevant != tt.wantRelevant {
				t.Errorf("Relevant = %v, want %v (reasons=%v)", got.Relevant, tt.wantRelevant, got.Reasons)
			}
			if tt.wantReason != "" && !slices.Contains(got.Reasons, tt.wantReason) {
				t.Errorf("Reasons = %v, want to contain %q", got.Reasons, tt.wantReason)
			}
			if tt.wantHint != "" && got.HintEventType != tt.wantHint {
				t.Errorf("HintEventType = %q, want %q", got.HintEventType, tt.wantHint)
			}
			if tt.wantScoreMore > 0 && got.Score < tt.wantScoreMore {
				t.Errorf("Score = %v, want >= %v", got.Score, tt.wantScoreMore)
			}
		})
	}
}

func TestFilter_IsRelevant_Dedup(t *testing.T) {
	cfg := baseConfig()
	clk := &fakeClock{now: fixedNow}
	f := New(cfg, clk)
	n := baseNews()

	first, err := f.IsRelevant(context.Background(), n)
	if err != nil {
		t.Fatalf("IsRelevant() error = %v", err)
	}
	if slices.Contains(first.Reasons, "duplicate within dedup window") {
		t.Fatalf("first call unexpectedly reported as duplicate: %v", first.Reasons)
	}

	second, err := f.IsRelevant(context.Background(), n)
	if err != nil {
		t.Fatalf("IsRelevant() error = %v", err)
	}
	if !slices.Contains(second.Reasons, "duplicate within dedup window") {
		t.Errorf("second call Reasons = %v, want to contain %q", second.Reasons, "duplicate within dedup window")
	}
	if second.Relevant {
		t.Errorf("second call Relevant = true, want false for duplicate")
	}

	clk.now = clk.now.Add(cfg.DedupWindow + time.Minute)

	third, err := f.IsRelevant(context.Background(), n)
	if err != nil {
		t.Fatalf("IsRelevant() error = %v", err)
	}
	if slices.Contains(third.Reasons, "duplicate within dedup window") {
		t.Errorf("third call after DedupWindow expired still reported as duplicate: %v", third.Reasons)
	}
}

func TestFilter_IsRelevant_ContextCanceled(t *testing.T) {
	f := New(baseConfig(), &fakeClock{now: fixedNow})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.IsRelevant(ctx, baseNews())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("IsRelevant() error = %v, want context.Canceled", err)
	}
}
