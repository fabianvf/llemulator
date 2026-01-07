package script

import (
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

// TestSimpleStringResponse tests a single string response
func TestSimpleStringResponse(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset:     true,
		Responses: "Hello, world!",
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	// First request should match
	response, err := engine.MatchRequest(token, "any message")
	if err != nil {
		t.Fatalf("Failed to match request: %v", err)
	}
	if response != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", response)
	}

	// Second request should fail (rule exhausted)
	_, err = engine.MatchRequest(token, "another message")
	if err == nil {
		t.Error("Expected error for exhausted rule")
	}
}

// TestSequentialResponses tests array of responses
func TestSequentialResponses(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset:     true,
		Responses: []interface{}{"First", "Second", "Third"},
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	// Test sequential matching
	expected := []string{"First", "Second", "Third"}
	for i, exp := range expected {
		response, err := engine.MatchRequest(token, "message "+string(rune(i)))
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if response != exp {
			t.Errorf("Request %d: expected '%s', got '%s'", i, exp, response)
		}
	}

	// Fourth request should fail
	_, err := engine.MatchRequest(token, "fourth message")
	if err == nil {
		t.Error("Expected error after responses exhausted")
	}
}

// TestPatternMatching verifies regex patterns work
func TestPatternMatching(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset: true,
		Responses: map[string]interface{}{
			".*hello.*":           "Hi there!",
			".*weather.*":         "It's sunny!",
			"\\d+\\s*\\+\\s*\\d+": "Math detected!",
			"exact match":         "Exact response",
		},
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	testCases := []struct {
		message  string
		expected string
	}{
		{"say hello please", "Hi there!"},
		{"hello world", "Hi there!"},
		{"what's the weather?", "It's sunny!"},
		{"2 + 2", "Math detected!"},
		{"exact match", "Exact response"},
		{"no match", ""}, // Should error
	}

	for _, tc := range testCases {
		response, err := engine.MatchRequest(token, tc.message)
		if tc.expected == "" {
			if err == nil {
				t.Errorf("Expected error for message '%s'", tc.message)
			}
		} else {
			if err != nil {
				t.Errorf("Failed to match '%s': %v", tc.message, err)
			}
			if response != tc.expected {
				t.Errorf("Message '%s': expected '%s', got '%s'", tc.message, tc.expected, response)
			}
		}
	}
}

// TestRuleCounters verifies times counter works
func TestRuleCounters(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset: true,
		Rules: []Rule{
			{
				Pattern:  "test",
				Response: "Limited response",
				Times:    2,
			},
			{
				Pattern:  "test",
				Response: "Unlimited response",
				Times:    -1, // Unlimited
			},
		},
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	// First two should match limited rule
	for i := 0; i < 2; i++ {
		response, err := engine.MatchRequest(token, "test")
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		if response != "Limited response" {
			t.Errorf("Request %d: expected limited response, got '%s'", i, response)
		}
	}

	// Subsequent requests should match unlimited rule
	for i := 0; i < 5; i++ {
		response, err := engine.MatchRequest(token, "test")
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+2, err)
		}
		if response != "Unlimited response" {
			t.Errorf("Request %d: expected unlimited response, got '%s'", i+2, response)
		}
	}
}

// TestMixedFormat tests mixing sequential and pattern responses
func TestMixedFormat(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset: true,
		Responses: []interface{}{
			"First default",
			map[string]interface{}{
				"pattern":  ".*help.*", // Make it a proper regex pattern
				"response": "Help response",
				"times":    2.0, // Ensure it's a float for proper JSON parsing
			},
			"Second default",
		},
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	// Rules are checked in order:
	// 1. "First default" with no pattern (matches anything, times=1)
	// 2. ".*help.*" pattern (times=2)
	// 3. "Second default" with no pattern (matches anything, times=1)

	// First request matches rule 1 (no pattern, matches anything)
	response, _ := engine.MatchRequest(token, "anything")
	if response != "First default" {
		t.Errorf("Expected 'First default', got '%s'", response)
	}

	// Now rule 1 is exhausted. "help" matches rule 2
	response, err := engine.MatchRequest(token, "help me")
	if err != nil {
		t.Fatalf("First help failed: %v", err)
	}
	if response != "Help response" {
		t.Errorf("Expected 'Help response', got '%s'", response)
	}

	// Second help still matches rule 2 (has 1 use left)
	response, err = engine.MatchRequest(token, "I need help")
	if err != nil {
		t.Fatalf("Second help failed: %v", err)
	}
	if response != "Help response" {
		t.Errorf("Expected 'Help response' again, got '%s'", response)
	}

	// Rule 2 is now exhausted. Next request matches rule 3
	response, _ = engine.MatchRequest(token, "other")
	if response != "Second default" {
		t.Errorf("Expected 'Second default', got '%s'", response)
	}

	// All rules exhausted, should error
	_, err = engine.MatchRequest(token, "final")
	if err == nil {
		t.Error("Expected error when all rules exhausted")
	}
}

// TestScriptReset verifies reset functionality
func TestScriptReset(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	// Load initial script
	script1 := Script{
		Reset:     true,
		Responses: "First script",
	}

	if err := engine.LoadScript(token, script1); err != nil {
		t.Fatalf("Failed to load first script: %v", err)
	}

	response, _ := engine.MatchRequest(token, "test")
	if response != "First script" {
		t.Errorf("First script not loaded correctly")
	}

	// Load second script with reset
	script2 := Script{
		Reset:     true,
		Responses: "Second script",
	}

	if err := engine.LoadScript(token, script2); err != nil {
		t.Fatalf("Failed to load second script: %v", err)
	}

	response, _ = engine.MatchRequest(token, "test")
	if response != "Second script" {
		t.Errorf("Second script not loaded correctly")
	}

	// Load third script without reset (should append)
	script3 := Script{
		Reset:     false,
		Responses: "Third script",
	}

	if err := engine.LoadScript(token, script3); err != nil {
		t.Fatalf("Failed to load third script: %v", err)
	}

	// Should still have second script rule
	response, _ = engine.MatchRequest(token, "test")
	if response != "Third script" {
		t.Errorf("Third script not appended correctly")
	}
}

// TestTokenIsolation verifies different tokens don't interfere
func TestTokenIsolation(t *testing.T) {
	engine := NewEngine()

	// Load different scripts for different tokens
	tokens := []string{"token1", "token2", "token3"}
	for i, token := range tokens {
		script := Script{
			Reset:     true,
			Responses: "Response for " + token,
		}
		if err := engine.LoadScript(token, script); err != nil {
			t.Fatalf("Failed to load script for %s: %v", token, err)
		}

		// Verify immediately
		response, err := engine.MatchRequest(token, "test")
		if err != nil {
			t.Fatalf("Failed to match for %s: %v", token, err)
		}
		expected := "Response for " + token
		if response != expected {
			t.Errorf("Token %d: expected '%s', got '%s'", i, expected, response)
		}
	}
}

// TestExtractUserMessage tests message extraction from request bodies
func TestExtractUserMessage(t *testing.T) {
	testCases := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "Chat completion format",
			body:     `{"messages":[{"role":"system","content":"Be helpful"},{"role":"user","content":"Hello world"}]}`,
			expected: "Hello world",
		},
		{
			name:     "Completion format",
			body:     `{"prompt":"Complete this text"}`,
			expected: "Complete this text",
		},
		{
			name:     "Input field format",
			body:     `{"input":"Process this input"}`,
			expected: "Process this input",
		},
		{
			name:     "Empty body",
			body:     ``,
			expected: "",
		},
		{
			name:     "Invalid JSON",
			body:     `{invalid}`,
			expected: "",
		},
		{
			name:     "No user message",
			body:     `{"messages":[{"role":"system","content":"Be helpful"}]}`,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractUserMessage([]byte(tc.body))
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// TestNoMatchError verifies helpful error messages
func TestNoMatchError(t *testing.T) {
	engine := NewEngine()
	token := "test-token"

	script := Script{
		Reset:     true,
		Responses: "Only response",
	}

	if err := engine.LoadScript(token, script); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	// Use the response
	engine.MatchRequest(token, "first")

	// Second should error with helpful message
	_, err := engine.MatchRequest(token, "test message")
	if err == nil {
		t.Fatal("Expected error for unmatched message")
	}

	if !strings.Contains(err.Error(), "test message") {
		t.Errorf("Error should contain the message that failed to match: %v", err)
	}
}
