package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is one turn in the chat-completions request.
// Role is "user" or "assistant". The system prompt is added by the client.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client calls the OpenAI-compatible chat endpoint in the environment.
type Client struct {
	key   string
	model string
	url   string
	http  *http.Client
}

// FromEnv reads API_KEY, TEXT_MODEL_NAME, and API_URL.
func FromEnv() (Client, error) {
	key := os.Getenv("API_KEY")
	model := os.Getenv("TEXT_MODEL_NAME")
	url := os.Getenv("API_URL")
	if key == "" || model == "" || url == "" {
		return Client{}, errors.New("set API_KEY, TEXT_MODEL_NAME, and API_URL in .env")
	}
	return Client{
		key:   key,
		model: model,
		url:   url,
		http:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends the system prompt followed by history, and returns the reply text.
func (c Client) Complete(ctx context.Context, history []Message) (string, error) {
	messages := make([]Message, 0, 1+len(history))
	messages = append(messages, Message{Role: "system", Content: SystemPrompt})
	messages = append(messages, history...)

	body, err := json.Marshal(request{Model: c.model, Messages: messages})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return readReply(res.StatusCode, raw)
}

// readReply pulls the assistant text out of a chat-completions body.
// An API error becomes a Go error so the chat can show the message.
func readReply(status int, body []byte) (string, error) {
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		if status < 200 || status >= 300 {
			return "", fmt.Errorf("model returned %d", status)
		}
		return "", fmt.Errorf("read model response: %w", err)
	}
	if resp.Error.Message != "" {
		return "", errors.New(resp.Error.Message)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("model returned %d", status)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("model returned an empty reply")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
