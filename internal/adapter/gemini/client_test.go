package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kite/internal/core"
)

func TestCompleteSendsToolsAndReadsToolCalls(t *testing.T) {
	var sent request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[
			{"id":"c1","function":{"name":"read_file","arguments":"{\"path\":\"main.go\"}"}}]}}]}`))
	}))
	defer server.Close()

	client := Client{key: "k", model: "m", url: server.URL}
	specs := []core.ToolSpec{{
		Name:        "read_file",
		Description: "Read a file",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}}

	reply, err := client.Complete(context.Background(), "system", []core.Message{{Role: "user", Message: "hi"}}, specs)
	if err != nil {
		t.Fatal(err)
	}

	if len(sent.Tools) != 1 || sent.Tools[0].Type != "function" || sent.Tools[0].Function.Name != "read_file" {
		t.Errorf("tools sent = %+v, want one read_file function", sent.Tools)
	}
	if len(reply.ToolCalls) != 1 || reply.ToolCalls[0] != (core.ToolCall{ID: "c1", Name: "read_file", Arguments: `{"path":"main.go"}`}) {
		t.Errorf("tool calls = %+v, want read_file c1", reply.ToolCalls)
	}
}

func TestCompleteWithoutToolsOmitsThem(t *testing.T) {
	var raw map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&raw)
		w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer server.Close()

	client := Client{key: "k", model: "m", url: server.URL}
	reply, err := client.Complete(context.Background(), "system", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := raw["tools"]; ok {
		t.Error("request has a tools field, want it left out when there are no tools")
	}
	if reply.Message != "hello" || len(reply.ToolCalls) != 0 {
		t.Errorf("reply = %+v, want plain text hello", reply)
	}
}
