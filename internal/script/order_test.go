package script

import (
	"encoding/json"
	"testing"
)

// The object form of "responses" documents that the first matching pattern
// wins. That only holds if the patterns keep the order they were written in,
// which means the raw JSON has to be walked rather than unmarshalled: a Go map
// randomises iteration, so a trailing ".*" catch-all would beat the specific
// patterns some fraction of the time. Loading the same script repeatedly is
// what catches a regression here, since a single load can get lucky.
func TestObjectResponsesKeepTheirOrder(t *testing.T) {
	const body = `{
	  "reset": true,
	  "responses": {
	    ".*hello.*world.*": "Hello world specific!",
	    ".*hello.*": "Hello general!",
	    ".*": "Default response"
	  }
	}`

	cases := []struct{ message, want string }{
		{"hello world", "Hello world specific!"},
		{"hello there", "Hello general!"},
		{"something else", "Default response"},
	}

	for attempt := 0; attempt < 50; attempt++ {
		var script Script
		if err := json.Unmarshal([]byte(body), &script); err != nil {
			t.Fatalf("decode script: %v", err)
		}
		engine := NewEngine()
		if err := engine.LoadScript("tok", script); err != nil {
			t.Fatalf("load script: %v", err)
		}
		for _, c := range cases {
			got, err := engine.MatchRequest("tok", c.message)
			if err != nil {
				t.Fatalf("attempt %d, %q: %v", attempt, c.message, err)
			}
			if got != c.want {
				t.Fatalf("attempt %d, %q: got %q, want %q",
					attempt, c.message, got, c.want)
			}
		}
	}
}

// A pattern may answer with a tool call rather than text, and that form has to
// survive the ordered path too.
func TestObjectResponsesCarryToolCalls(t *testing.T) {
	const body = `{
	  "reset": true,
	  "responses": {
	    ".*list.*": {"tool_calls": [{"id": "call_1", "type": "function",
	                 "function": {"name": "shell",
	                              "arguments": "{\"command\":\"ls -1\"}"}}]},
	    ".*": "done"
	  }
	}`

	var script Script
	if err := json.Unmarshal([]byte(body), &script); err != nil {
		t.Fatalf("decode script: %v", err)
	}
	engine := NewEngine()
	if err := engine.LoadScript("tok", script); err != nil {
		t.Fatalf("load script: %v", err)
	}

	rule, err := engine.MatchRule("tok", "please list the files")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(rule.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(rule.ToolCalls))
	}
	if rule.ToolCalls[0].Function.Name != "shell" {
		t.Fatalf("got tool %q, want shell", rule.ToolCalls[0].Function.Name)
	}
}
