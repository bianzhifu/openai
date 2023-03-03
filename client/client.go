package client

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

var (
	API_KEY              = ""
	Model_gpt35turbo     = "gpt-3.5-turbo"
	Model_gpt35turbo0301 = "gpt-3.5-turbo-0301"
)

func InitApi(apikey string) {
	API_KEY = apikey
}

type ChatRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string               `json:"model"`
	Messages []ChatRequestMessage `json:"messages"`
}

type ChatRespone struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
		Message      struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"message"`
	} `json:"choices"`
	Created int    `json:"created"`
	ID      string `json:"id"`
	Object  string `json:"object"`
	Usage   struct {
		CompletionTokens int `json:"completion_tokens"`
		PromptTokens     int `json:"prompt_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func Chat(chatReq *ChatRequest) (error, *ChatRespone) {
	client := &http.Client{}
	reqdata, err := json.Marshal(chatReq)
	if err != nil {
		return err, nil
	}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqdata))
	if err != nil {
		return err, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+API_KEY)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, nil
	}
	var chatResp ChatRespone
	err = json.Unmarshal(bodyText, &chatResp)
	if err != nil {
		log.Fatal(err)
	}
	return nil, &chatResp
}
