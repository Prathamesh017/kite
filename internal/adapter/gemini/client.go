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
	Tools    []tool    `json:"tools,omitempty"`
}

// tool is one tool in the provider's format.
type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type toolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type response struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (c Client) Complete(ctx context.Context, systemPrompt string, history []core.Message, specs []core.ToolSpec) (core.Message, error) {
	messages := []message{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		messages = append(messages, message{Role: m.Role, Content: m.Message})
	}

	var tools []tool
	for _, s := range specs {
		tools = append(tools, tool{
			Type:     "function",
			Function: toolFunction{Name: s.Name, Description: s.Description, Parameters: s.Parameters},
		})
	}

	body, err := json.Marshal(request{Model: c.model, Messages: messages, Tools: tools})
	if err != nil {
		return core.Message{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return core.Message{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return core.Message{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return core.Message{}, fmt.Errorf("model returned %d", res.StatusCode)
	}

	var reply response
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return core.Message{}, err
	}
	if len(reply.Choices) == 0 {
		return core.Message{}, errors.New("model returned an empty reply")
	}
	got := reply.Choices[0].Message
	out := core.Message{Role: "assistant", Message: got.Content}
	for _, call := range got.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, core.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return out, nil
}
