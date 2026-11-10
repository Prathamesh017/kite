package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"kite/internal/core"
)

// var _ core.LLM = Client{}

type Client struct {
	key   string
	model string
	url   string
	http  *http.Client // nil means http.DefaultClient
	retry retry
}

func FromEnv() (Client, error) {
	c := Client{
		key:   os.Getenv("API_KEY"),
		model: os.Getenv("TEXT_MODEL_NAME"),
		url:   os.Getenv("API_URL"),
		http:  &http.Client{Timeout: requestTimeout()},
		retry: retryFromEnv(),
	}
	if c.key == "" || c.model == "" || c.url == "" {
		return Client{}, errors.New("set API_KEY, TEXT_MODEL_NAME, and API_URL in .env")
	}
	return c, nil
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type request struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	Tools          []tool          `json:"tools,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// responseFormat asks the model to answer in a JSON shape.
type responseFormat struct {
	Type       string `json:"type"`
	JSONSchema struct {
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
	} `json:"json_schema"`
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
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	// ExtraContent holds Gemini's thought signature, which it expects back unchanged.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type response struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (c Client) Complete(ctx context.Context, systemPrompt string, history []core.Message, specs []core.ToolSpec, format *core.ReplyFormat) (core.Message, error) {
	messages := []message{{Role: "system", Content: systemPrompt}}
	for _, m := range history {
		wire := message{Role: m.Role, Content: m.Message, ToolCallID: m.ToolCallID}
		for _, call := range m.ToolCalls {
			sent := toolCall{ID: call.ID, Type: "function", ExtraContent: call.ExtraContent}
			sent.Function.Name = call.Name
			sent.Function.Arguments = call.Arguments
			wire.ToolCalls = append(wire.ToolCalls, sent)
		}
		messages = append(messages, wire)
	}

	var tools []tool
	for _, s := range specs {
		tools = append(tools, tool{
			Type:     "function",
			Function: toolFunction{Name: s.Name, Description: s.Description, Parameters: s.Parameters},
		})
	}

	var wire *responseFormat
	if format != nil {
		wire = &responseFormat{Type: "json_schema"}
		wire.JSONSchema.Name = format.Name
		wire.JSONSchema.Schema = format.Schema
	}

	body, err := json.Marshal(request{Model: c.model, Messages: messages, Tools: tools, ResponseFormat: wire})
	if err != nil {
		return core.Message{}, err
	}

	reply, err := c.send(ctx, body)
	// A failure that might pass is sent again, up to the retry count, waiting a
	// little longer each time. It stops early when the error cannot be retried.
	for attempt := 0; err != nil && retryable(err) && attempt < c.retry.count; attempt++ {
		select {
		case <-ctx.Done():
			return core.Message{}, ctx.Err()
		case <-time.After(randomUpTo(backoff(attempt, c.retry))):
		}
		reply, err = c.send(ctx, body)
	}
	if err != nil {
		return core.Message{}, err
	}
	if len(reply.Choices) == 0 {
		return core.Message{}, errors.New("model returned an empty reply")
	}
	got := reply.Choices[0].Message
	out := core.Message{Role: "assistant", Message: got.Content}
	for _, call := range got.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, core.ToolCall{
			ID:           call.ID,
			Name:         call.Function.Name,
			Arguments:    call.Function.Arguments,
			ExtraContent: call.ExtraContent,
		})
	}
	return out, nil
}

// send makes one request. A reply other than 200 comes back as an *APIError.
func (c Client) send(ctx context.Context, body []byte) (response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	client := c.http
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return response{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		// The body says why the request was rejected, so keep the start of it.
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return response{}, &APIError{Status: res.StatusCode, Message: strings.TrimSpace(string(detail))}
	}

	var reply response
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return response{}, err
	}
	return reply, nil
}
