package weighted

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"trading-bot/internal/application/config"
	"trading-bot/internal/domain/news"
	"trading-bot/internal/domain/trading"
)

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

var fixedNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func testConfig() config.OrchestratorConfig {
	return config.OrchestratorConfig{
		Version:                "v1",
		Weights:                config.Weights{Sentiment: 0.6, Surprise: 0.4},
		UnknownSurprisePenalty: 0,
		MixedSentimentPenalty:  0.5,
		EventModifiers: map[string]float64{
			"earnings": 1.0,
			"other":    0.5,
		},
		Thresholds: map[string]config.ThresholdSet{
			"intraday": {MinConfidence: 0.6, MinImpact: 0.5, BuyThreshold: 0.5, SellThreshold: 0.5, WatchThreshold: 0.3, SignalTTLSeconds: 3600, CooldownSeconds: 1800},
			"swing":    {MinConfidence: 0.5, MinImpact: 0.4, BuyThreshold: 0.4, SellThreshold: 0.4, WatchThreshold: 0.25, SignalTTLSeconds: 259200, CooldownSeconds: 86400},
			"position": {MinConfidence: 0.5, MinImpact: 0.4, BuyThreshold: 0.35, SellThreshold: 0.35, WatchThreshold: 0.2, SignalTTLSeconds: 2592000, CooldownSeconds: 604800},
		},
	}
}

func mustScore(t *testing.T, v float64) news.Score {
	t.Helper()

	s, err := news.NewScore(v)
	if err != nil {
		t.Fatalf("NewScore(%v) error = %v", v, err)
	}
	return s
}

func tickerPtr(tk trading.Ticker) *trading.Ticker {
	return &tk
}

func TestEngine_Decide(t *testing.T) {
	const scoreEpsilon = 1e-9

	tests := []struct {
		name          string
		analysis      func(t *testing.T) news.NewsAnalysis
		wantAction    trading.Action
		wantTrack     trading.Track
		wantTicker    trading.Ticker
		wantFinalNear *float64 // если задан — FinalScore сверяется с точностью scoreEpsilon
	}{
		{
			name: "positive sentiment and surprise on intraday horizon triggers buy",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-1",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.HorizonIntraday,
				}
			},
			wantAction: trading.ActionBuy,
			wantTrack:  trading.TrackIntraday,
			wantTicker: "SBER",
		},
		{
			name: "negative sentiment and surprise on swing horizon triggers sell",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-2",
					Ticker:               tickerPtr("GAZP"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentNegative,
					Surprise:             news.SurpriseNegative,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.Horizon1To3Days,
				}
			},
			wantAction: trading.ActionSell,
			wantTrack:  trading.TrackSwing,
			wantTicker: "GAZP",
		},
		{
			name: "moderate positive score with unknown event modifier falls back to 1 and yields watch",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-3",
					Ticker:               tickerPtr("LKOH"),
					EventType:            news.EventTypeManagement, // нет в EventModifiers testConfig -> fallback 1
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurpriseUnknown,
					ImpactScore:          mustScore(t, 0.6),
					Confidence:           mustScore(t, 0.7),
					TimeHorizon:          news.Horizon1To3Months,
				}
			},
			wantAction:    trading.ActionWatch,
			wantTrack:     trading.TrackPosition,
			wantTicker:    "LKOH",
			wantFinalNear: floatPtr(0.6 * 0.6 * 0.7 * 1), // directionalRaw=0.6*1 (surprise unknown -> вес 0)
		},
		{
			name: "confidence below MinConfidence forces hold regardless of score",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-4",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.3), // < intraday MinConfidence 0.6
					TimeHorizon:          news.HorizonIntraday,
				}
			},
			wantAction: trading.ActionHold,
			wantTrack:  trading.TrackIntraday,
			wantTicker: "SBER",
		},
		{
			name: "impact below MinImpact forces hold regardless of score",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-5",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.2), // < intraday MinImpact 0.5
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.HorizonIntraday,
				}
			},
			wantAction: trading.ActionHold,
			wantTrack:  trading.TrackIntraday,
			wantTicker: "SBER",
		},
		{
			name: "unknown time horizon has no thresholds and forces hold",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-6",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.HorizonUnknown,
				}
			},
			wantAction: trading.ActionHold,
			wantTrack:  trading.TrackUnknown,
			wantTicker: "SBER",
		},
		{
			name: "nil ticker forces watch even though score would trigger buy",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-7",
					Ticker:               nil,
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentPositive,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.HorizonIntraday,
				}
			},
			wantAction: trading.ActionWatch,
			wantTrack:  trading.TrackIntraday,
			wantTicker: "",
		},
		{
			name: "mixed sentiment score is dampened by MixedSentimentPenalty",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-8",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentMixed,
					PositiveFactors:      []string{"a", "b", "c"},
					NegativeFactors:      []string{"d"},
					Surprise:             news.SurpriseNeutral,
					ImpactScore:          mustScore(t, 0.8),
					Confidence:           mustScore(t, 0.8),
					TimeHorizon:          news.Horizon1To3Days,
				}
			},
			wantAction:    trading.ActionHold,
			wantTrack:     trading.TrackSwing,
			wantTicker:    "SBER",
			wantFinalNear: floatPtr(0.6 * ((3.0 - 1.0) / (3.0 + 1.0) * 0.5) * 0.8 * 0.8),
		},
		{
			name: "mixed sentiment with no factors is neutral, not a division by zero",
			analysis: func(t *testing.T) news.NewsAnalysis {
				return news.NewsAnalysis{
					NewsID:               "news-9",
					Ticker:               tickerPtr("SBER"),
					EventType:            news.EventTypeEarnings,
					FundamentalSentiment: news.SentimentMixed,
					Surprise:             news.SurprisePositive,
					ImpactScore:          mustScore(t, 0.9),
					Confidence:           mustScore(t, 0.9),
					TimeHorizon:          news.HorizonIntraday,
				}
			},
			wantAction:    trading.ActionWatch,
			wantTrack:     trading.TrackIntraday,
			wantTicker:    "SBER",
			wantFinalNear: floatPtr(0.4 * 0.9 * 0.9),
		},
	}

	engine := New(testConfig(), fakeClock{now: fixedNow})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := tt.analysis(t)

			got, err := engine.Decide(context.Background(), analysis)
			if err != nil {
				t.Fatalf("Decide() error = %v", err)
			}

			if got.Action != tt.wantAction {
				t.Errorf("Action = %q, want %q (final_score=%v)", got.Action, tt.wantAction, got.FinalScore)
			}
			if got.Track != tt.wantTrack {
				t.Errorf("Track = %q, want %q", got.Track, tt.wantTrack)
			}
			if got.Ticker != tt.wantTicker {
				t.Errorf("Ticker = %q, want %q", got.Ticker, tt.wantTicker)
			}
			if tt.wantFinalNear != nil && math.Abs(got.FinalScore-*tt.wantFinalNear) > scoreEpsilon {
				t.Errorf("FinalScore = %v, want %v", got.FinalScore, *tt.wantFinalNear)
			}
			wantStrength := math.Min(1, math.Abs(got.FinalScore))
			if math.Abs(got.Strength-wantStrength) > scoreEpsilon {
				t.Errorf("Strength = %v, want %v", got.Strength, wantStrength)
			}
			if got.NewsID != analysis.NewsID {
				t.Errorf("NewsID = %q, want %q", got.NewsID, analysis.NewsID)
			}
			if got.Status != trading.SignalStatusPending {
				t.Errorf("Status = %q, want %q", got.Status, trading.SignalStatusPending)
			}
			if !got.GeneratedAt.Equal(fixedNow) {
				t.Errorf("GeneratedAt = %v, want %v", got.GeneratedAt, fixedNow)
			}
		})
	}
}

func TestEngine_Decide_SignalIDIsDeterministic(t *testing.T) {
	analysis := news.NewsAnalysis{
		NewsID:               "news-1",
		Ticker:               tickerPtr("SBER"),
		EventType:            news.EventTypeEarnings,
		FundamentalSentiment: news.SentimentPositive,
		Surprise:             news.SurprisePositive,
		ImpactScore:          mustScore(t, 0.9),
		Confidence:           mustScore(t, 0.9),
		TimeHorizon:          news.HorizonIntraday,
	}

	engineV1a := New(testConfig(), fakeClock{now: fixedNow})
	engineV1b := New(testConfig(), fakeClock{now: fixedNow.Add(time.Hour)})

	first, err := engineV1a.Decide(context.Background(), analysis)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	second, err := engineV1b.Decide(context.Background(), analysis)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("SignalID differs across calls with the same NewsID/engine/config version: %q vs %q", first.ID, second.ID)
	}

	cfgV2 := testConfig()
	cfgV2.Version = "v2"
	engineV2 := New(cfgV2, fakeClock{now: fixedNow})

	third, err := engineV2.Decide(context.Background(), analysis)
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	if first.ID == third.ID {
		t.Errorf("SignalID did not change when config Version changed: %q", first.ID)
	}
}

func TestEngine_Decide_ContextCanceled(t *testing.T) {
	engine := New(testConfig(), fakeClock{now: fixedNow})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.Decide(ctx, news.NewsAnalysis{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Decide() error = %v, want context.Canceled", err)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
