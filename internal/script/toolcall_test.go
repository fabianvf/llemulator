package script

import (
	"encoding/json"
	"testing"

	"github.com/fabianvf/llemulator/internal/models"
)

// TestToolCallRule tests that a rule can answer with a tool call
func TestToolCallRule(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	err := engine.LoadScript(token, Script{
		Reset: true,
		Rules: []Rule{{
			Pattern: "list the files",
			Times:   1,
			ToolCalls: []models.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: models.ToolCallFunction{
					Name:      "shell",
					Arguments: `{"command":"ls"}`,
				},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	rule, err := engine.MatchRule(token, "please list the files")
	if err != nil {
		t.Fatalf("MatchRule: %v", err)
	}
	if len(rule.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(rule.ToolCalls))
	}
	if rule.ToolCalls[0].Function.Name != "shell" {
		t.Errorf("function = %q, want shell", rule.ToolCalls[0].Function.Name)
	}
}

// TestToolCallFromSimplifiedFormat tests declaring a tool call in the array form
func TestToolCallFromSimplifiedFormat(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	var responses interface{}
	if err := json.Unmarshal([]byte(`[
		{"pattern": "run it", "tool_calls": [
			{"id": "call_1", "type": "function",
			 "function": {"name": "shell", "arguments": "{\"command\":\"echo hi\"}"}}
		]}
	]`), &responses); err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadScript(token, Script{Reset: true, Responses: responses}); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	rule, err := engine.MatchRule(token, "please run it")
	if err != nil {
		t.Fatalf("MatchRule: %v", err)
	}
	if len(rule.ToolCalls) != 1 || rule.ToolCalls[0].Function.Arguments != `{"command":"echo hi"}` {
		t.Fatalf("tool call did not survive the script format: %+v", rule.ToolCalls)
	}
}

// TestToolCallDefaultsToFiringOnce tests that a client answering a tool call
// does not get the same call again forever.
//
// The client sends the result back and the last user message is unchanged, so
// an unlimited rule would match a second time and the conversation would never
// end. The tool-call rule is spent after one use and the next rule answers.
func TestToolCallDefaultsToFiringOnce(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	var responses interface{}
	if err := json.Unmarshal([]byte(`[
		{"pattern": "run it", "tool_calls": [
			{"id": "call_1", "type": "function",
			 "function": {"name": "shell", "arguments": "{}"}}
		]},
		{"pattern": "", "response": "all done", "times": -1}
	]`), &responses); err != nil {
		t.Fatal(err)
	}
	if err := engine.LoadScript(token, Script{Reset: true, Responses: responses}); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	first, err := engine.MatchRule(token, "please run it")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatal("first reply should be the tool call")
	}

	second, err := engine.MatchRule(token, "please run it")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second.ToolCalls) != 0 {
		t.Error("the tool call repeated, so the conversation would not terminate")
	}
	if second.Response != "all done" {
		t.Errorf("second reply = %q, want the next rule", second.Response)
	}
}

// TestTextRulesCarryNoToolCalls tests that ordinary responses are unchanged
func TestTextRulesCarryNoToolCalls(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	if err := engine.LoadScript(token, Script{Reset: true, Responses: "just text"}); err != nil {
		t.Fatal(err)
	}
	rule, err := engine.MatchRule(token, "anything")
	if err != nil {
		t.Fatal(err)
	}
	if len(rule.ToolCalls) != 0 {
		t.Errorf("a text rule produced tool calls: %+v", rule.ToolCalls)
	}
	// And the old accessor still works for callers that only want content.
	content, err := engine.MatchRequest(token, "anything")
	if err == nil && content != "" {
		t.Errorf("MatchRequest returned %q after the rule was spent", content)
	}
}
