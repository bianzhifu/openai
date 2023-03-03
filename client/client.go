package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

var (
	API_KEY   = ""
	Model     = ""
	CuzClient *http.Client
)

func InitCuzClient(proxy string) {
	proxyUrl, _ := url.Parse("socks5://" + proxy)
	if len(proxy) > 0 {
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
		CuzClient = &http.Client{Transport: transport}
	}
	CuzClient = &http.Client{}
}

func InitApi(apikey string, model string) {
	API_KEY = apikey
	Model = model
}

type ChatRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string               `json:"model"`
	Messages    []ChatRequestMessage `json:"messages"`
	Temperature float64              `json:"temperature"`
	User        string               `json:"user"`
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
	defer func() {
		if err := recover(); err != nil {
			log.Printf("run Chat time panic: %v", err)
		}
	}()
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
	resp, err := CuzClient.Do(req)
	if err != nil {
		return err, nil
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return err, nil
	}
	var chatResp ChatRespone
	err = json.Unmarshal(bodyText, &chatResp)
	if err != nil {
		fmt.Println(string(bodyText))
		return err, nil
	}
	return nil, &chatResp
}
