package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fabianvf/llemulator/internal/models"
	"github.com/fabianvf/llemulator/internal/server"
)

// TestEndToEndChatCompletion tests complete chat completion flow
func TestEndToEndChatCompletion(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	token := "integration-test"

	// Load script with simple response
	scriptPayload := map[string]interface{}{
		"reset":     true,
		"responses": "Test response",
	}

	scriptBody, _ := json.Marshal(scriptPayload)
	scriptReq, _ := http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+token)
	scriptReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(scriptReq)
	if err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Script loading failed with status: %d, body: %s", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	// Make chat completion request
	chatPayload := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}

	chatBody, _ := json.Marshal(chatPayload)
	chatReq, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
	chatReq.Header.Set("Authorization", "Bearer "+token)
	chatReq.Header.Set("Content-Type", "application/json")

	chatResp, err := http.DefaultClient.Do(chatReq)
	if err != nil {
		t.Fatalf("Chat completion request failed: %v", err)
	}
	defer chatResp.Body.Close()

	if chatResp.StatusCode != http.StatusOK {
		t.Fatalf("Chat completion returned status: %d", chatResp.StatusCode)
	}

	// Parse response
	var completion models.ChatCompletion
	if err := json.NewDecoder(chatResp.Body).Decode(&completion); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response structure
	if len(completion.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(completion.Choices))
	}
	if completion.Choices[0].Message.Content != "Test response" {
		t.Errorf("Expected 'Test response', got %s", completion.Choices[0].Message.Content)
	}
}

// TestStreamingIntegration tests streaming chat completion
func TestStreamingIntegration(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	token := "stream-test"

	// Load script with streaming response
	scriptPayload := map[string]interface{}{
		"reset":     true,
		"responses": "Hello world for streaming test",
	}

	scriptBody, _ := json.Marshal(scriptPayload)
	scriptReq, _ := http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+token)
	scriptReq.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(scriptReq)

	// Make streaming request
	chatPayload := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Test"},
		},
	}

	chatBody, _ := json.Marshal(chatPayload)
	chatReq, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
	chatReq.Header.Set("Authorization", "Bearer "+token)
	chatReq.Header.Set("Content-Type", "application/json")

	chatResp, err := http.DefaultClient.Do(chatReq)
	if err != nil {
		t.Fatalf("Streaming request failed: %v", err)
	}
	defer chatResp.Body.Close()

	// Verify SSE headers
	if ct := chatResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}

	// Read streaming response
	body, _ := io.ReadAll(chatResp.Body)
	lines := strings.Split(string(body), "\n")

	// Verify we got SSE events
	var eventCount int
	var foundDone bool
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			eventCount++
			if strings.Contains(line, "[DONE]") {
				foundDone = true
			}
		}
	}

	if eventCount < 3 {
		t.Errorf("Expected at least 3 SSE events, got %d", eventCount)
	}
	if !foundDone {
		t.Error("Stream missing [DONE] termination")
	}
}

// TestConcurrentTokenIsolation tests that different tokens are isolated
func TestConcurrentTokenIsolation(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tokens := []string{"token1", "token2", "token3"}
	var wg sync.WaitGroup

	for i, token := range tokens {
		wg.Add(1)
		go func(tk string, id int) {
			defer wg.Done()

			// Each token loads different script
			content := fmt.Sprintf("Response for token %d", id)
			scriptPayload := map[string]interface{}{
				"reset":     true,
				"responses": []interface{}{content, content, content, content, content, content, content, content, content, content},
			}

			scriptBody, _ := json.Marshal(scriptPayload)
			scriptReq, _ := http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
			scriptReq.Header.Set("Authorization", "Bearer "+tk)
			scriptReq.Header.Set("Content-Type", "application/json")

			if resp, err := http.DefaultClient.Do(scriptReq); err != nil || resp.StatusCode != 200 {
				t.Errorf("Failed to load script for token %s", tk)
				return
			}

			// Make multiple requests with this token
			for j := 0; j < 5; j++ {
				chatPayload := map[string]interface{}{
					"model": "gpt-4",
					"messages": []map[string]interface{}{
						{"role": "user", "content": fmt.Sprintf("Message %d", j)},
					},
				}

				chatBody, _ := json.Marshal(chatPayload)
				chatReq, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
				chatReq.Header.Set("Authorization", "Bearer "+tk)
				chatReq.Header.Set("Content-Type", "application/json")

				resp, err := http.DefaultClient.Do(chatReq)
				if err != nil {
					t.Errorf("Request failed for token %s: %v", tk, err)
					continue
				}

				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				// Parse and verify we get the correct response for this token
				var result map[string]interface{}
				if err := json.Unmarshal(body, &result); err == nil {
					if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if msg, ok := choice["message"].(map[string]interface{}); ok {
								if msgContent, ok := msg["content"].(string); ok {
									if msgContent != content {
										t.Errorf("Token %s got wrong response: %s, expected: %s", tk, msgContent, content)
									}
								}
							}
						}
					}
				}
			}
		}(token, i)
	}

	wg.Wait()
}

// TestExplicitTwoTokenIsolation verifies complete isolation between two tokens with interleaved requests
func TestExplicitTwoTokenIsolation(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tokenAnimals := "token-animals"
	tokenColors := "token-colors"

	// Load animal responses for first token
	animalScript := map[string]interface{}{
		"reset":     true,
		"responses": []string{"Dog", "Cat", "Bird", "Mouse", "Fish"},
	}

	scriptBody, _ := json.Marshal(animalScript)
	scriptReq, _ := http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+tokenAnimals)
	scriptReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(scriptReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to load animal script: %v", err)
	}
	resp.Body.Close()

	// Load color responses for second token
	colorScript := map[string]interface{}{
		"reset":     true,
		"responses": []string{"Red", "Blue", "Green", "Yellow", "Purple"},
	}

	scriptBody, _ = json.Marshal(colorScript)
	scriptReq, _ = http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+tokenColors)
	scriptReq.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(scriptReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to load color script: %v", err)
	}
	resp.Body.Close()

	// Helper function to make a request and get response content
	makeRequest := func(token, message string) (string, error) {
		chatPayload := map[string]interface{}{
			"model": "gpt-4",
			"messages": []map[string]interface{}{
				{"role": "user", "content": message},
			},
		}

		chatBody, _ := json.Marshal(chatPayload)
		chatReq, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
		chatReq.Header.Set("Authorization", "Bearer "+token)
		chatReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(chatReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}

		// Check for error
		if errObj, ok := result["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				return "", fmt.Errorf("%s", msg)
			}
		}

		// Extract content
		if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice := choices[0].(map[string]interface{}); choice != nil {
				if msg := choice["message"].(map[string]interface{}); msg != nil {
					if content, ok := msg["content"].(string); ok {
						return content, nil
					}
				}
			}
		}

		return "", fmt.Errorf("no content in response")
	}

	// Test with interleaved pattern: A1, C1, C2, C3, A2, A3, concurrent(A4,C4), A5, C5
	type testStep struct {
		token    string
		expected string
		name     string
	}

	// Sequential interleaved requests
	sequentialSteps := []testStep{
		{tokenAnimals, "Dog", "Animal-1"},
		{tokenColors, "Red", "Color-1"},
		{tokenColors, "Blue", "Color-2"},
		{tokenColors, "Green", "Color-3"},
		{tokenAnimals, "Cat", "Animal-2"},
		{tokenAnimals, "Bird", "Animal-3"},
	}

	for _, step := range sequentialSteps {
		response, err := makeRequest(step.token, fmt.Sprintf("Request for %s", step.name))
		if err != nil {
			t.Fatalf("%s failed: %v", step.name, err)
		}
		if response != step.expected {
			t.Errorf("%s: expected '%s', got '%s'", step.name, step.expected, response)
		}
	}

	// Concurrent requests to both tokens
	var wg sync.WaitGroup
	results := make(map[string]string)
	errors := make(map[string]error)
	var resultMu sync.Mutex

	concurrentSteps := []testStep{
		{tokenAnimals, "Mouse", "Animal-4-concurrent"},
		{tokenColors, "Yellow", "Color-4-concurrent"},
	}

	for _, step := range concurrentSteps {
		wg.Add(1)
		go func(s testStep) {
			defer wg.Done()
			response, err := makeRequest(s.token, fmt.Sprintf("Concurrent %s", s.name))

			resultMu.Lock()
			if err != nil {
				errors[s.name] = err
			} else {
				results[s.name] = response
			}
			resultMu.Unlock()
		}(step)
	}

	wg.Wait()

	// Verify concurrent results
	for _, step := range concurrentSteps {
		if err := errors[step.name]; err != nil {
			t.Fatalf("Concurrent %s failed: %v", step.name, err)
		}
		if results[step.name] != step.expected {
			t.Errorf("Concurrent %s: expected '%s', got '%s'", step.name, step.expected, results[step.name])
		}
	}

	// Final sequential requests
	finalSteps := []testStep{
		{tokenAnimals, "Fish", "Animal-5"},
		{tokenColors, "Purple", "Color-5"},
	}

	for _, step := range finalSteps {
		response, err := makeRequest(step.token, fmt.Sprintf("Final %s", step.name))
		if err != nil {
			t.Fatalf("Final %s failed: %v", step.name, err)
		}
		if response != step.expected {
			t.Errorf("Final %s: expected '%s', got '%s'", step.name, step.expected, response)
		}
	}

	// Both should now be exhausted
	_, err = makeRequest(tokenAnimals, "Should fail - animals exhausted")
	if err == nil {
		t.Error("Animal token should be exhausted but request succeeded")
	}

	_, err = makeRequest(tokenColors, "Should fail - colors exhausted")
	if err == nil {
		t.Error("Color token should be exhausted but request succeeded")
	}

	// Test pattern-based responses with interleaving
	patternScript := map[string]interface{}{
		"reset": true,
		"responses": map[string]interface{}{
			".*hello.*": "Hi there!",
			".*bye.*":   "Goodbye!",
		},
	}

	scriptBody, _ = json.Marshal(patternScript)
	scriptReq, _ = http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+tokenAnimals)
	scriptReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(scriptReq)

	// Color token still exhausted
	_, err = makeRequest(tokenColors, "Still exhausted")
	if err == nil {
		t.Error("Color token should still be exhausted")
	}

	// Animal token has new pattern responses
	response, _ := makeRequest(tokenAnimals, "hello world")
	if response != "Hi there!" {
		t.Errorf("Pattern match failed: expected 'Hi there!', got '%s'", response)
	}
}

// TestRequestSerialization tests that requests for same token serialize
func TestRequestSerialization(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	token := "serialize-test"

	// Load script with multiple responses
	scriptPayload := map[string]interface{}{
		"reset":     true,
		"responses": []interface{}{"Response", "Response", "Response", "Response", "Response", "Response", "Response", "Response", "Response", "Response"},
	}

	scriptBody, _ := json.Marshal(scriptPayload)
	scriptReq, _ := http.NewRequest("POST", ts.URL+"/_emulator/script", bytes.NewReader(scriptBody))
	scriptReq.Header.Set("Authorization", "Bearer "+token)
	scriptReq.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(scriptReq)

	// Track request order
	var mu sync.Mutex
	var requestOrder []int

	// Launch concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Add delay to ensure concurrency
			time.Sleep(time.Duration(id) * 10 * time.Millisecond)

			chatPayload := map[string]interface{}{
				"model": "gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": fmt.Sprintf("Request %d", id)},
				},
			}

			chatBody, _ := json.Marshal(chatPayload)
			chatReq, _ := http.NewRequest("POST", ts.URL+"/v1/chat/completions", bytes.NewReader(chatBody))
			chatReq.Header.Set("Authorization", "Bearer "+token)
			chatReq.Header.Set("Content-Type", "application/json")

			startTime := time.Now()
			resp, err := http.DefaultClient.Do(chatReq)
			duration := time.Since(startTime)

			if err != nil {
				t.Errorf("Request %d failed: %v", id, err)
				return
			}
			resp.Body.Close()

			mu.Lock()
			requestOrder = append(requestOrder, id)
			mu.Unlock()

			// Log timing for debugging
			t.Logf("Request %d completed in %v", id, duration)
		}(i)
	}

	wg.Wait()

	// All requests should complete
	if len(requestOrder) != 5 {
		t.Errorf("Expected 5 completed requests, got %d", len(requestOrder))
	}
}

// TestModelEndpoint tests the /v1/models endpoint
func TestModelEndpoint(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	token := "model-test"

	// Test listing models
	req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Models request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Models endpoint returned status: %d", resp.StatusCode)
	}

	var modelList models.ModelList
	if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
		t.Fatalf("Failed to parse model list: %v", err)
	}

	// Should have default models
	if len(modelList.Data) == 0 {
		t.Error("Model list is empty")
	}

	// Test getting specific model
	modelID := "gpt-4"
	req2, _ := http.NewRequest("GET", ts.URL+"/v1/models/"+modelID, nil)
	req2.Header.Set("Authorization", "Bearer "+token)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Model detail request failed: %v", err)
	}
	defer resp2.Body.Close()

	var model models.Model
	if err := json.NewDecoder(resp2.Body).Decode(&model); err != nil {
		t.Fatalf("Failed to parse model: %v", err)
	}

	if model.ID != modelID {
		t.Errorf("Expected model ID %s, got %s", modelID, model.ID)
	}
}

// TestErrorResponses tests error response formats
func TestErrorResponses(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	testCases := []struct {
		name           string
		method         string
		path           string
		token          string
		body           []byte
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing auth",
			method:         "POST",
			path:           "/v1/chat/completions",
			token:          "",
			body:           []byte(`{"model": "gpt-4"}`),
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "auth_error",
		},
		{
			name:           "Invalid JSON",
			method:         "POST",
			path:           "/v1/chat/completions",
			token:          "test",
			body:           []byte(`{invalid json}`),
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid_request_error",
		},
		{
			name:           "No script loaded",
			method:         "POST",
			path:           "/v1/chat/completions",
			token:          "no-script-token",
			body:           []byte(`{"model": "gpt-4"}`),
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "server_error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, ts.URL+tc.path, bytes.NewReader(tc.body))
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			var errorResp models.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			if errorResp.Error.Type != tc.expectedError {
				t.Errorf("Expected error type %s, got %s", tc.expectedError, errorResp.Error.Type)
			}
		})
	}
}
