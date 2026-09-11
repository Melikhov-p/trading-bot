package memory

import (
	"context"
	"testing"

	"trading-bot/internal/domain/news"
)

func TestAnalysisRepository_Save(t *testing.T) {
	r := NewAnalysisRepository()

	a := news.NewsAnalysis{NewsID: "n-1", Company: "Sber"}
	if err := r.Save(context.Background(), a); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestAnalysisRepository_RespectsCancelledContext(t *testing.T) {
	r := NewAnalysisRepository()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := r.Save(ctx, news.NewsAnalysis{}); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
