package yandexgpt

// promptDTO — тело поля "prompt" запроса к YandexGPT Responses API.
type promptDTO struct {
	ID        string            `json:"id"`
	Variables map[string]string `json:"variables,omitempty"`
}

// responseRequestDTO — тело запроса к YandexGPT Responses API.
type responseRequestDTO struct {
	Prompt promptDTO `json:"prompt"`
	Input  string    `json:"input"`
}

// responseDataDTO — конверт ответа YandexGPT Responses API.
type responseDataDTO struct {
	OutputText string `json:"output_text"`
}

// analysisDTO — тело output_text, распарсенное по JSON-схеме
// prompts/news_analysis.ru.txt.
type analysisDTO struct {
	SchemaVersion        string   `json:"schema_version"`
	Company              string   `json:"company"`
	Ticker               *string  `json:"ticker"`
	EventType            string   `json:"event_type"`
	FundamentalSentiment string   `json:"fundamental_sentiment"`
	Surprise             string   `json:"surprise"`
	ImpactScore          float64  `json:"impact_score"`
	Confidence           float64  `json:"confidence"`
	TimeHorizon          string   `json:"time_horizon"`
	PositiveFactors      []string `json:"positive_factors"`
	NegativeFactors      []string `json:"negative_factors"`
	KeyRisk              string   `json:"key_risk"`
	Reason               string   `json:"reason"`
}
