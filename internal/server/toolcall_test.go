package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fabianvf/llemulator/internal/models"
	"github.com/fabianvf/llemulator/internal/script"
)

const toolCallScript = `[
  {"pattern": "list the files", "tool_calls": [
    {"id": "call_1", "type": "function",
     "function": {"name": "shell", "arguments": "{\"command\":\"ls -1\"}"}}
  ]},
  {"pattern": "", "response": "all done", "times": -1}
]`

func loadToolCallScript(t *testing.T, s *Server, token string) {
	t.Helper()
	var responses interface{}
	if err := json.Unmarshal([]byte(toolCallScript), &responses); err != nil {
		t.Fatal(err)
	}
	if err := s.engine.LoadScript(token, script.Script{Reset: true, Responses: responses}); err != nil {
		t.Fatal(err)
	}
}

func chatRequest(t *testing.T, s *Server, token, message string, stream bool) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"model":    "gpt-4",
		"stream":   stream,
		"messages": []map[string]string{{"role": "user", "content": message}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

// TestChatCompletionReturnsToolCall tests the non-streaming wire shape a client
// consumes: tool_calls on the assistant message and finish_reason tool_calls.
func TestChatCompletionReturnsToolCall(t *testing.T) {
	s := NewServer()
	loadToolCallScript(t, s, "tok")

	rec := chatRequest(t, s, "tok", "please list the files", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var completion models.ChatCompletion
	if err := json.Unmarshal(rec.Body.Bytes(), &completion); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	choice := completion.Choices[0]
	if choice.FinishReason == nil || *choice.FinishReason != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %+v", choice.Message.ToolCalls)
	}
	call := choice.Message.ToolCalls[0]
	if call.Type != "function" || call.Function.Name != "shell" {
		t.Errorf("call = %+v", call)
	}
	// Arguments stay a JSON string, which is what SDKs hand to a JSON parser.
	var args map[string]string
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		t.Fatalf("arguments are not a JSON document: %v", err)
	}
	if args["command"] != "ls -1" {
		t.Errorf("arguments = %v", args)
	}
}

// TestStreamedToolCall tests that the SSE path carries the call and finishes
// with tool_calls, since a client that streams must see the same thing.
func TestStreamedToolCall(t *testing.T) {
	s := NewServer()
	loadToolCallScript(t, s, "tok")

	rec := chatRequest(t, s, "tok", "please list the files", true)
	body := rec.Body.String()

	if !strings.Contains(body, `"tool_calls"`) {
		t.Fatalf("stream carried no tool call:\n%s", body)
	}
	if !strings.Contains(body, `"finish_reason":"tool_calls"`) {
		t.Errorf("stream did not finish with tool_calls:\n%s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Error("stream did not terminate")
	}
}

// TestTextResponseIsUnchanged tests that adding tool calls did not alter an
// ordinary reply: no tool_calls key at all, and finish_reason still stop.
func TestTextResponseIsUnchanged(t *testing.T) {
	s := NewServer()
	if err := s.engine.LoadScript("tok", script.Script{Reset: true, Responses: "just text"}); err != nil {
		t.Fatal(err)
	}

	rec := chatRequest(t, s, "tok", "anything", false)
	body := rec.Body.String()
	if strings.Contains(body, "tool_calls") {
		t.Errorf("a text reply carried a tool_calls key:\n%s", body)
	}
	if !strings.Contains(body, `"finish_reason":"stop"`) {
		t.Errorf("finish_reason changed for text replies:\n%s", body)
	}
}

// TestToolResultIsAccepted tests that a client can send the result back. The
// assistant turn has null content with tool_calls and the result arrives as a
// role:"tool" message, neither of which existed in the request shape before.
func TestToolResultIsAccepted(t *testing.T) {
	s := NewServer()
	loadToolCallScript(t, s, "tok")

	body, err := json.Marshal(map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "please list the files"},
			{"role": "assistant", "content": nil, "tool_calls": []map[string]interface{}{
				{"id": "call_1", "type": "function",
					"function": map[string]string{"name": "shell", "arguments": "{}"}},
			}},
			{"role": "tool", "tool_call_id": "call_1", "content": "a.txt\nb.txt"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("a tool result was rejected with %d: %s", rec.Code, rec.Body.String())
	}
}
