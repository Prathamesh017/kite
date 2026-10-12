package llm

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

	"kite/internal/tools"
)

// Message is one turn in the chat-completions request.
// Role is "user", "assistant", or "tool". The system prompt is added by the client.
// An assistant message asking for tools carries ToolCalls. A tool message
// carries the result in Content and the request it answers in ToolCallID.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Client calls the OpenAI-compatible chat endpoint in the environment.
type Client struct {
	key   string
	model string
	url   string
	http  *http.Client
}

// FromEnv reads API_KEY, TEXT_MODEL_NAME, API_URL, and REQUEST_TIMEOUT.
// A missing or unreadable timeout falls back to 60s.
func FromEnv() (Client, error) {
	key := os.Getenv("API_KEY")
	model := os.Getenv("TEXT_MODEL_NAME")
	url := os.Getenv("API_URL")
	if key == "" || model == "" || url == "" {
		return Client{}, errors.New("set API_KEY, TEXT_MODEL_NAME, and API_URL in .env")
	}
	timeout := 60 * time.Second
	if d, err := time.ParseDuration(os.Getenv("REQUEST_TIMEOUT")); err == nil && d > 0 {
		timeout = d
	}
	return Client{
		key:   key,
		model: model,
		url:   url,
		http:  &http.Client{Timeout: timeout},
	}, nil
}

type request struct {
	Model          string            `json:"model"`
	Messages       []Message         `json:"messages"`
	Tools          []json.RawMessage `json:"tools"`
	ResponseFormat json.RawMessage   `json:"response_format"`
}

// ToolCall is the model asking to run one tool. The result sent back must
// carry the same ID. ExtraContent holds Gemini's signature, which it expects
// to get back unchanged on the next request.
type ToolCall struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Function     FunctionCall    `json:"function"`
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

// FunctionCall is which tool to run. Arguments is a JSON string, for example {"path":"main.go"}.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type response struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends the system prompt followed by history.
// The tool definitions and the JSON reply shape go with every request, like
// the system prompt. It returns the reply text (JSON in the reply shape, see
// ParseReply), the finish reason ("stop" for an answer, "tool_calls" when the
// model wants tools run), and the tool calls.
func (c Client) Complete(ctx context.Context, history []Message) (string, string, []ToolCall, error) {
	messages := make([]Message, 0, 1+len(history))
	messages = append(messages, Message{Role: "system", Content: SystemPrompt})
	messages = append(messages, history...)

	body, err := json.Marshal(request{
		Model:          c.model,
		Messages:       messages,
		Tools:          tools.Definitions,
		ResponseFormat: json.RawMessage(replyFormat),
	})
	if err != nil {
		return "", "", nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", "", nil, &APIError{}
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", nil, &APIError{}
	}
	return readReply(res.StatusCode, raw)
}

// readReply pulls the text, finish reason, and tool calls out of a
// chat-completions body. Any failure comes back as an *APIError carrying the status code.
func readReply(status int, body []byte) (string, string, []ToolCall, error) {
	var resp response
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", nil, &APIError{Status: status}
	}
	if resp.Error.Message != "" {
		return "", "", nil, &APIError{Status: status, Message: resp.Error.Message}
	}
	if status < 200 || status >= 300 {
		return "", "", nil, &APIError{Status: status}
	}
	if len(resp.Choices) == 0 {
		return "", "", nil, &APIError{Status: status, Message: "model returned an empty reply"}
	}

	choice := resp.Choices[0]
	calls := choice.Message.ToolCalls
	text := strings.TrimSpace(choice.Message.Content)
	// A tool request has no text, so it is only empty when there are no calls either.
	if text == "" && len(calls) == 0 {
		return "", "", nil, &APIError{Status: status, Message: "model returned an empty reply"}
	}
	return text, choice.FinishReason, calls, nil
}
