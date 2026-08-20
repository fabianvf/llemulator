package script

import (
	"testing"

	"github.com/fabianvf/llemulator/internal/models"
)

// A rule written without "times" used to arrive as 0, which the matcher skips
// as exhausted, so the rule never fired and every request failed.
func TestExplicitRuleWithoutTimesIsUnlimited(t *testing.T) {
	e := NewEngine()
	if err := e.LoadScript("tok", Script{
		Rules: []Rule{{Pattern: "", Response: "always"}},
	}); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	for i := range 3 {
		rule, err := e.MatchRule("tok", "anything")
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if rule.Response != "always" {
			t.Fatalf("call %d: got %q", i+1, rule.Response)
		}
	}
}

func TestExplicitToolCallRuleDefaultsToOnce(t *testing.T) {
	e := NewEngine()
	if err := e.LoadScript("tok", Script{
		Rules: []Rule{
			{Pattern: "run it", ToolCalls: []models.ToolCall{{ID: "call_1", Type: "function"}}},
			{Pattern: "", Response: "done"},
		},
	}); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	first, err := e.MatchRule("tok", "run it")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first.ToolCalls) != 1 {
		t.Fatalf("first call should return the tool call, got %+v", first)
	}

	// Same message again: the tool-call rule is spent, so the catch-all ends it.
	second, err := e.MatchRule("tok", "run it")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(second.ToolCalls) != 0 || second.Response != "done" {
		t.Fatalf("tool-call rule fired twice: %+v", second)
	}
}

func TestExplicitTimesIsRespected(t *testing.T) {
	e := NewEngine()
	if err := e.LoadScript("tok", Script{
		Rules: []Rule{{Pattern: "", Response: "once", Times: 1}, {Pattern: "", Response: "after"}},
	}); err != nil {
		t.Fatalf("LoadScript: %v", err)
	}

	if r, _ := e.MatchRule("tok", "x"); r.Response != "once" {
		t.Fatalf("got %q", r.Response)
	}
	if r, _ := e.MatchRule("tok", "x"); r.Response != "after" {
		t.Fatalf("times: 1 was not honoured, got %q", r.Response)
	}
}
