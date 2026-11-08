package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"kite/internal/core"
)

// var _ core.LLM = Client{}

type Client struct {
	key   string
	model string
	url   string
}

func FromEnv() (Client, error) {
	c := Client{
		key:   os.Getenv("API_KEY"),
		model: os.Getenv("TEXT_MODEL_NAME"),
		url:   os.Getenv("API_URL"),
	}
	if c.key == "" || c.model == "" || c.url == "" {
		return Client{}, errors.New("set API_KEY, TEXT_MODEL_NAME, and API_URL in .env")
	}
	return c, nil
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type response struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func (c Client) Complete(ctx context.Context, systemPrompt string, history []core.Message) (string, error) {
	messages := []message{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		messages = append(messages, message{Role: m.Role, Content: m.Message})
	}

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

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("model returned %d", res.StatusCode)
	}

	var reply response
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return "", err
	}
	if len(reply.Choices) == 0 {
		return "", errors.New("model returned an empty reply")
	}
	return reply.Choices[0].Message.Content, nil
}
