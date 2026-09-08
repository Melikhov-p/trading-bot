package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Prompt struct {
	ID        string            `json:"id"`
	Variables map[string]string `json:"variables,omitempty"`
}

type ResponseRequest struct {
	Prompt Prompt `json:"prompt"`
	Input  string `json:"input"`
}

type ResponseData struct {
	OutputText string `json:"output_text"`
}

func main() {
	apiKey := os.Getenv("YANDEX_API_KEY")
	folderID := os.Getenv("YANDEX_FOLDER_ID")
	if apiKey == "" || folderID == "" {
		log.Fatal("YANDEX_API_KEY and YANDEX_FOLDER_ID environment variables must be set")
	}

	reqData := ResponseRequest{
		Prompt: Prompt{
			ID: "fvtpln2v5hgd3etr4hhf",
		},
		Input: "Сбербанк сообщил, что чистая прибыль по итогам III квартала\n2026 года составила 510 млрд рублей, увеличившись на 24% год к году.\n\nКонсенсус-прогноз аналитиков перед публикацией отчётности составлял\n470 млрд рублей.\n\nROE составил 25,8% против 23,1% годом ранее.\n\nЧистый процентный доход увеличился на 18%.\n\nСтоимость риска выросла с 1,2% до 1,5%.\n\nБанк сохранил прогноз по росту чистой прибыли за 2026 год\nна уровне 10–15%.\n\nАкции Сбербанка выросли на 4,2% за последние пять торговых дней.",
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", "https://ai.api.cloud.yandex.net/v1/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+apiKey)
	req.Header.Set("OpenAI-Project", folderID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Response: %s\n", string(body))

	if resp.StatusCode == 200 {
		var response ResponseData
		err = json.Unmarshal(body, &response)
		if err != nil {
			log.Printf("Error parsing response: %v", err)
		} else {
			fmt.Println("Output:", response.OutputText)
		}
	}
}
