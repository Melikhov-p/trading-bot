package yandexgpt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"trading-bot/internal/application/port"
	"trading-bot/internal/domain/news"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata %q: %v", name, err)
	}

	return string(data)
}

func TestParseAnalysis(t *testing.T) {
	t.Run("valid fixture", func(t *testing.T) {
		outputText := readTestdata(t, "test.json")

		analysis, err := parseAnalysis("news-1", "yandexgpt", outputText, time.Unix(0, 0))
		if err != nil {
			t.Fatalf("parseAnalysis() error = %v", err)
		}

		if analysis.Company != "Сбербанк" {
			t.Errorf("Company = %q, want %q", analysis.Company, "Сбербанк")
		}
		if analysis.EventType != news.EventTypeEarnings {
			t.Errorf("EventType = %q, want %q", analysis.EventType, news.EventTypeEarnings)
		}
		if analysis.FundamentalSentiment != news.SentimentPositive {
			t.Errorf("FundamentalSentiment = %q, want %q", analysis.FundamentalSentiment, news.SentimentPositive)
		}
		if analysis.Surprise != news.SurprisePositive {
			t.Errorf("Surprise = %q, want %q", analysis.Surprise, news.SurprisePositive)
		}
		if analysis.TimeHorizon != news.Horizon1To3Months {
			t.Errorf("TimeHorizon = %q, want %q", analysis.TimeHorizon, news.Horizon1To3Months)
		}
		if analysis.ImpactScore.Float64() != 0.8 {
			t.Errorf("ImpactScore = %v, want 0.8", analysis.ImpactScore.Float64())
		}
		if analysis.Ticker != nil {
			t.Errorf("Ticker = %v, want nil", analysis.Ticker)
		}
		if analysis.NewsID != "news-1" {
			t.Errorf("NewsID = %q, want %q", analysis.NewsID, "news-1")
		}
	})

	t.Run("markdown wrapped fixture", func(t *testing.T) {
		outputText := readTestdata(t, "test_markdown_wrapped.json")

		analysis, err := parseAnalysis("news-1", "yandexgpt", outputText, time.Unix(0, 0))
		if err != nil {
			t.Fatalf("parseAnalysis() error = %v", err)
		}

		if analysis.Company != "Сбербанк" {
			t.Errorf("Company = %q, want %q", analysis.Company, "Сбербанк")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := parseAnalysis("news-1", "yandexgpt", "this is not json at all", time.Unix(0, 0))
		if !errors.Is(err, port.ErrInvalidJSON) {
			t.Fatalf("parseAnalysis() error = %v, want port.ErrInvalidJSON", err)
		}
	})

	t.Run("schema violation on unknown event type", func(t *testing.T) {
		outputText := `{"company":"X","ticker":null,"event_type":"unknown_event",` +
			`"fundamental_sentiment":"positive","surprise":"unknown","impact_score":0.5,` +
			`"confidence":0.5,"time_horizon":"intraday","positive_factors":[],"negative_factors":[],` +
			`"key_risk":"","reason":""}`

		_, err := parseAnalysis("news-1", "yandexgpt", outputText, time.Unix(0, 0))
		if !errors.Is(err, port.ErrSchemaViolation) {
			t.Fatalf("parseAnalysis() error = %v, want port.ErrSchemaViolation", err)
		}
	})

	t.Run("schema violation on out of range score", func(t *testing.T) {
		outputText := `{"company":"X","ticker":null,"event_type":"earnings",` +
			`"fundamental_sentiment":"positive","surprise":"unknown","impact_score":1.5,` +
			`"confidence":0.5,"time_horizon":"intraday","positive_factors":[],"negative_factors":[],` +
			`"key_risk":"","reason":""}`

		_, err := parseAnalysis("news-1", "yandexgpt", outputText, time.Unix(0, 0))
		if !errors.Is(err, port.ErrSchemaViolation) {
			t.Fatalf("parseAnalysis() error = %v, want port.ErrSchemaViolation", err)
		}
	})
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, Config) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := Config{
		APIKey:   "test-key",
		FolderID: "test-folder",
		PromptID: "test-prompt",
		Timeout:  time.Second,
	}

	return srv, cfg
}

func TestAnalyzer_Analyze(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		outputText := readTestdata(t, "test.json")

		srv, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(responseDataDTO{OutputText: outputText})
		})

		a := New(cfg, srv.Client(), fakeClock{now: time.Unix(100, 0)})

		a.endpoint = srv.URL
		a.retryDelay = time.Millisecond

		analysis, err := a.Analyze(context.Background(), news.News{ID: "news-1", Body: "тестовая новость"})
		if err != nil {
			t.Fatalf("Analyze() error = %v", err)
		}
		if analysis.Company != "Сбербанк" {
			t.Errorf("Company = %q, want %q", analysis.Company, "Сбербанк")
		}
		if !analysis.AnalyzedAt.Equal(time.Unix(100, 0)) {
			t.Errorf("AnalyzedAt = %v, want %v", analysis.AnalyzedAt, time.Unix(100, 0))
		}
	})

	t.Run("retries on 500 then succeeds", func(t *testing.T) {
		outputText := readTestdata(t, "test.json")

		attempts := 0
		srv, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(responseDataDTO{OutputText: outputText})
		})

		a := New(cfg, srv.Client(), fakeClock{now: time.Unix(100, 0)})

		a.endpoint = srv.URL
		a.retryDelay = time.Millisecond

		_, err := a.Analyze(context.Background(), news.News{ID: "news-1", Body: "тестовая новость"})
		if err != nil {
			t.Fatalf("Analyze() error = %v", err)
		}
		if attempts != 2 {
			t.Errorf("attempts = %d, want 2", attempts)
		}
	})

	t.Run("does not retry on 400", func(t *testing.T) {
		attempts := 0
		srv, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
		})

		a := New(cfg, srv.Client(), fakeClock{now: time.Unix(100, 0)})

		a.endpoint = srv.URL
		a.retryDelay = time.Millisecond

		_, err := a.Analyze(context.Background(), news.News{ID: "news-1", Body: "тестовая новость"})
		if err == nil {
			t.Fatal("Analyze() error = nil, want error")
		}
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("gives up after exhausting retries on persistent 503", func(t *testing.T) {
		attempts := 0
		srv, cfg := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		a := New(cfg, srv.Client(), fakeClock{now: time.Unix(100, 0)})

		a.endpoint = srv.URL
		a.retryDelay = time.Millisecond

		_, err := a.Analyze(context.Background(), news.News{ID: "news-1", Body: "тестовая новость"})
		if !errors.Is(err, port.ErrUpstreamUnavailable) {
			t.Fatalf("Analyze() error = %v, want port.ErrUpstreamUnavailable", err)
		}
		if attempts != maxRetries+1 {
			t.Errorf("attempts = %d, want %d", attempts, maxRetries+1)
		}
	})
}
