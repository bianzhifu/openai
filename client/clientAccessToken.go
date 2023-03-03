package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strings"
)

var (
	AccessTokenModel = "text-davinci-002-render-sha"
	UserAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	EOF_TEXT         = "[DONE]"
	AccessToken      = ""
	ReverseProxyURL  = ""
	//https://chat.duti.tech/api/conversation
	//https://gpt.pawan.krd/backend-api/conversation
)

type AccessTokenAuthor struct {
	Role string `json:"role"`
}

type AccessTokenContent struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

type AccessTokenMessage struct {
	ID      string             `json:"id"`
	Author  AccessTokenAuthor  `json:"author"`
	Role    string             `json:"role"`
	Content AccessTokenContent `json:"content"`
}

type RequestAccessToken struct {
	Action          string               `json:"action"`
	Messages        []AccessTokenMessage `json:"messages"`
	ConversationID  string               `json:"conversation_id,omitempty"`
	ParentMessageID string               `json:"parent_message_id"`
	Model           string               `json:"model"`
}

type ChatText struct {
	data           string // event data
	ConversationID string // conversation context id
	MessageID      string // current message id, can used as next chat's parent_message_id
	Content        string // text content
}

func sendMessage(message string, args ...string) (*http.Response, error) {

	var messageID string
	var conversationID string
	var parentMessageID string

	messageID = uuid.NewString()
	if len(args) > 0 {
		conversationID = args[0]
	}
	if len(args) > 1 {
		parentMessageID = args[1]
	}
	if parentMessageID == "" {
		parentMessageID = uuid.NewString()
	}

	params := RequestAccessToken{
		Action:          "next",
		Model:           AccessTokenModel,
		ParentMessageID: parentMessageID,
		Messages: []AccessTokenMessage{{
			Author: AccessTokenAuthor{
				Role: "user",
			},
			Role: "user",
			ID:   messageID,
			Content: AccessTokenContent{
				ContentType: "text",
				Parts:       []string{message},
			},
		},
		},
	}
	if conversationID != "" {
		params.ConversationID = conversationID
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, ReverseProxyURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("new request failed: %v", err)
	}

	bearerToken := fmt.Sprintf("Bearer %s", AccessToken)
	req.Header.Set("Authorization", bearerToken)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")

	resp, err := CuzClient.Do(req)

	return resp, err
}

func parseChatText(text string) (*ChatText, error) {
	if text == "" || text == EOF_TEXT {
		return nil, fmt.Errorf("invalid chat text: %s", text)
	}
	json, err := simplejson.NewFromReader(strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("invalid json parse: %s", text)
	}
	conversationID, _ := json.Get("conversation_id").String()
	messageID, _ := json.Get("message").Get("id").String()
	content, _ := json.Get("message").Get("content").Get("parts").GetIndex(0).String()

	if conversationID == "" || messageID == "" {
		return nil, fmt.Errorf("invalid chat text")
	}

	return &ChatText{
		data:           text,
		ConversationID: conversationID,
		MessageID:      messageID,
		Content:        content,
	}, nil
}

func GetChatText(message string, args ...string) (*ChatText, error) {
	resp, err := sendMessage(message, args...)
	if err != nil {
		return nil, fmt.Errorf("send message failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %v", err)
	}

	arr := strings.Split(string(body), "\n\n")

	const TEXT_ARR_MIN_LEN = 3
	const TEXT_STR_MIN_LEN = 6

	l := len(arr)
	if l < TEXT_ARR_MIN_LEN {
		return nil, fmt.Errorf("invalid reply message: %s", body)
	}

	str := arr[l-TEXT_ARR_MIN_LEN]
	if len(str) < TEXT_STR_MIN_LEN {
		return nil, fmt.Errorf("invalid reply message: %s", body)
	}

	text := str[TEXT_STR_MIN_LEN:]

	return parseChatText(text)
}
