package news

import "fmt"

// Score — value object с инвариантом [0,1]. Используется для impact_score
// и confidence из ответа NewsAnalyzer.
type Score float64

// NewScore проверяет инвариант диапазона и возвращает валидный Score.
func NewScore(v float64) (Score, error) {
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("news: score must be in [0,1], got %v", v)
	}
	return Score(v), nil
}

// Float64 возвращает значение Score как float64.
func (s Score) Float64() float64 {
	return float64(s)
}
