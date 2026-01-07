package script

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNewEngine tests engine creation
func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

// TestExactMatchPriority verifies exact matches beat patterns
func TestExactMatchPriority(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
					Pattern: ".*hello.*", // Pattern that would match
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Pattern response",
				},
			},
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
					JSON:   json.RawMessage(`{"model": "gpt-4", "messages": [{"content": "hello world"}]}`),
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Exact match response",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Request that matches both rules
	body := []byte(`{"model": "gpt-4", "messages": [{"content": "hello world"}]}`)
	
	response, err := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("Failed to match request: %v", err)
	}
	
	// Should prefer exact match over pattern
	if response.Content != "Exact match response" {
		t.Errorf("Expected exact match to take priority, got: %s", response.Content)
	}
}

// TestPatternMatching verifies regex patterns work correctly
func TestPatternMatching(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	testCases := []struct {
		name    string
		pattern string
		message string
		shouldMatch bool
	}{
		{"Simple match", "hello", "hello world", true},
		{"No match", "goodbye", "hello world", false},
		{"Regex match", "^hello.*world$", "hello beautiful world", true},
		{"Case insensitive", "(?i)HELLO", "hello", true},
		{"Number pattern", "\\d+", "I have 42 apples", true},
		{"Email pattern", "[a-z]+@[a-z]+\\.[a-z]+", "email: test@example.com", true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			script := Script{
				Reset: true,
				Rules: []Rule{
					{
						Match: MatchRule{
							Pattern: tc.pattern,
						},
						Times: 1,
						Response: ResponseRule{
							Status:  200,
							Content: "Matched!",
						},
					},
				},
			}
			
			if err := engine.LoadScript(token, script); err != nil {
				t.Fatalf("Failed to load script: %v", err)
			}
			
			body := []byte(`{"messages": [{"role": "user", "content": "` + tc.message + `"}]}`)
			response, err := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
			
			if tc.shouldMatch {
				if err != nil {
					t.Errorf("Expected match but got error: %v", err)
				}
				if response == nil || response.Content != "Matched!" {
					t.Error("Expected pattern to match but it didn't")
				}
			} else {
				if err == nil && response != nil {
					t.Error("Expected no match but got a response")
				}
			}
		})
	}
}

// TestJSONSubsetMatching verifies partial JSON matches work
func TestJSONSubsetMatching(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
					JSON:   json.RawMessage(`{"model": "gpt-4"}`), // Only match model
				},
				Times: 2,
				Response: ResponseRule{
					Status:  200,
					Content: "GPT-4 response",
				},
			},
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
					JSON:   json.RawMessage(`{"model": "gpt-3.5-turbo"}`),
				},
				Times: 2,
				Response: ResponseRule{
					Status:  200,
					Content: "GPT-3.5 response",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Request with gpt-4 and extra fields
	body1 := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "test"}],
		"temperature": 0.7,
		"max_tokens": 100
	}`)
	
	response1, err := engine.MatchRequest(token, "POST", "/v1/chat/completions", body1)
	if err != nil {
		t.Fatalf("Failed to match request: %v", err)
	}
	if response1.Content != "GPT-4 response" {
		t.Errorf("Expected GPT-4 response, got: %s", response1.Content)
	}
	
	// Request with gpt-3.5-turbo
	body2 := []byte(`{
		"model": "gpt-3.5-turbo",
		"messages": [{"role": "user", "content": "different test"}]
	}`)
	
	response2, err := engine.MatchRequest(token, "POST", "/v1/chat/completions", body2)
	if err != nil {
		t.Fatalf("Failed to match request: %v", err)
	}
	if response2.Content != "GPT-3.5 response" {
		t.Errorf("Expected GPT-3.5 response, got: %s", response2.Content)
	}
}

// TestRuleCounters verifies rules decrement properly
func TestRuleCounters(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
				},
				Times: 2, // Should match exactly 2 times
				Response: ResponseRule{
					Status:  200,
					Content: "Limited response",
				},
			},
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
				},
				Times: -1, // Unlimited
				Response: ResponseRule{
					Status:  200,
					Content: "Fallback response",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	body := []byte(`{"model": "gpt-4"}`)
	
	// First request - should match first rule
	response1, _ := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
	if response1.Content != "Limited response" {
		t.Errorf("First request should match limited rule, got: %s", response1.Content)
	}
	
	// Second request - should still match first rule
	response2, _ := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
	if response2.Content != "Limited response" {
		t.Errorf("Second request should match limited rule, got: %s", response2.Content)
	}
	
	// Third request - first rule exhausted, should match fallback
	response3, _ := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
	if response3.Content != "Fallback response" {
		t.Errorf("Third request should match fallback rule, got: %s", response3.Content)
	}
}

// TestUnmatchedBehavior verifies unmatched requests always error
func TestUnmatchedBehavior(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/specific",
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Specific response",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Request that doesn't match any rule should always error
	body := []byte(`{"test": "unmatched"}`)
	response, err := engine.MatchRequest(token, "POST", "/v1/unmatched", body)
	
	if err == nil {
		t.Error("Expected error for unmatched request")
	}
	if response != nil {
		t.Error("Expected no response for unmatched request")
	}
	
	// Error message should be helpful
	if err != nil && !strings.Contains(err.Error(), "no matching rule") {
		t.Errorf("Error message not helpful: %v", err)
	}
}

// TestResponseFormats verifies all response types supported
func TestResponseFormats(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			// Plain text response
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/text",
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Plain text response",
				},
			},
			// JSON response
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/json",
				},
				Times: 1,
				Response: ResponseRule{
					Status: 200,
					JSON:   json.RawMessage(`{"result": "success", "data": {"id": 123}}`),
				},
			},
			// SSE response
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/stream",
				},
				Times: 1,
				Response: ResponseRule{
					Status: 200,
					SSE: []SSEEvent{
						{Data: json.RawMessage(`{"chunk": 1}`)},
						{Data: json.RawMessage(`{"chunk": 2}`)},
						{Data: json.RawMessage(`"[DONE]"`)},
					},
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Test text response
	response1, _ := engine.MatchRequest(token, "POST", "/v1/text", nil)
	if response1.Content != "Plain text response" {
		t.Errorf("Expected text response, got: %s", response1.Content)
	}
	
	// Test JSON response
	response2, _ := engine.MatchRequest(token, "POST", "/v1/json", nil)
	if response2.JSON == nil {
		t.Error("Expected JSON response")
	}
	
	// Test SSE response
	response3, _ := engine.MatchRequest(token, "POST", "/v1/stream", nil)
	if len(response3.SSE) != 3 {
		t.Errorf("Expected 3 SSE events, got: %d", len(response3.SSE))
	}
}

// TestScriptReset verifies script reset functionality
func TestScriptReset(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	// Load initial script
	script1 := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "GET",
					Path:   "/v1/test",
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "First script",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script1); err != nil {
		t.Fatalf("Failed to load first script: %v", err)
	}
	
	// Verify first script works
	response1, _ := engine.MatchRequest(token, "GET", "/v1/test", nil)
	if response1.Content != "First script" {
		t.Errorf("First script not loaded correctly")
	}
	
	// Load second script with reset
	script2 := Script{
		Reset: true, // Should clear previous rules
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/different",
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Second script",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script2); err != nil {
		t.Fatalf("Failed to load second script: %v", err)
	}
	
	// Old rule should not match
	response2, err := engine.MatchRequest(token, "GET", "/v1/test", nil)
	if err == nil && response2 != nil {
		t.Error("Old rule should not match after reset")
	}
	
	// New rule should match
	response3, _ := engine.MatchRequest(token, "POST", "/v1/different", nil)
	if response3.Content != "Second script" {
		t.Errorf("Second script not loaded correctly")
	}
}

// TestComplexJSONMatching tests nested JSON matching
func TestComplexJSONMatching(t *testing.T) {
	engine := NewEngine()
	token := "test-token"
	
	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
					JSON: json.RawMessage(`{
						"messages": [
							{"role": "system", "content": "You are helpful"},
							{"role": "user"}
						]
					}`),
				},
				Times: 1,
				Response: ResponseRule{
					Status:  200,
					Content: "Matched complex structure",
				},
			},
		},
	}
	
	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Request with matching structure plus extra fields
	body := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "system", "content": "You are helpful"},
			{"role": "user", "content": "Hello world"}
		],
		"temperature": 0.5
	}`)
	
	response, err := engine.MatchRequest(token, "POST", "/v1/chat/completions", body)
	if err != nil {
		t.Fatalf("Failed to match request: %v", err)
	}
	if response.Content != "Matched complex structure" {
		t.Errorf("Complex JSON matching failed")
	}
}