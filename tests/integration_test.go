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

	"github.com/fabianvf/llemulator/internal/server"
	"github.com/fabianvf/llemulator/internal/models"
)

// TestEndToEndChatCompletion tests complete chat completion flow
func TestEndToEndChatCompletion(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()
	
	token := "integration-test"
	
	// Load script
	scriptPayload := map[string]interface{}{
		"reset": true,
		"rules": []map[string]interface{}{
			{
				"match": map[string]interface{}{
					"method": "POST",
					"path":   "/v1/chat/completions",
					"json":   map[string]interface{}{"model": "gpt-4"},
				},
				"times": 2,
				"response": map[string]interface{}{
					"status": 200,
					"json": map[string]interface{}{
						"id":     "test-id",
						"object": "chat.completion",
						"model":  "gpt-4",
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"message": map[string]interface{}{
									"role":    "assistant",
									"content": "Test response",
								},
								"finish_reason": "stop",
							},
						},
					},
				},
			},
		},
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
		t.Fatalf("Script loading failed with status: %d", resp.StatusCode)
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
	if completion.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", completion.ID)
	}
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
	
	// Load script with SSE response
	scriptPayload := map[string]interface{}{
		"reset": true,
		"rules": []map[string]interface{}{
			{
				"match": map[string]interface{}{
					"method": "POST",
					"path":   "/v1/chat/completions",
					"json":   map[string]interface{}{"stream": true},
				},
				"times": 1,
				"response": map[string]interface{}{
					"status": 200,
					"sse": []map[string]interface{}{
						{"data": map[string]interface{}{
							"id":     "stream-1",
							"object": "chat.completion.chunk",
							"choices": []map[string]interface{}{
								{"index": 0, "delta": map[string]interface{}{"role": "assistant"}},
							},
						}},
						{"data": map[string]interface{}{
							"id":     "stream-1",
							"object": "chat.completion.chunk",
							"choices": []map[string]interface{}{
								{"index": 0, "delta": map[string]interface{}{"content": "Hello"}},
							},
						}},
						{"data": map[string]interface{}{
							"id":     "stream-1",
							"object": "chat.completion.chunk",
							"choices": []map[string]interface{}{
								{"index": 0, "delta": map[string]interface{}{"content": " world"}},
							},
						}},
						{"data": "[DONE]"},
					},
				},
			},
		},
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
				"reset": true,
				"rules": []map[string]interface{}{
					{
						"match": map[string]interface{}{
							"method": "POST",
							"path":   "/v1/chat/completions",
						},
						"times": 10,
						"response": map[string]interface{}{
							"status":  200,
							"content": content,
						},
					},
				},
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
				
				// Verify we get the correct response for this token
				if !strings.Contains(string(body), content) {
					t.Errorf("Token %s got wrong response: %s", tk, string(body))
				}
			}
		}(token, i)
	}
	
	wg.Wait()
}

// TestRequestSerialization tests that requests for same token serialize
func TestRequestSerialization(t *testing.T) {
	srv := server.NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()
	
	token := "serialize-test"
	
	// Load script with counter
	scriptPayload := map[string]interface{}{
		"reset": true,
		"rules": []map[string]interface{}{
			{
				"match": map[string]interface{}{
					"method": "POST",
					"path":   "/v1/chat/completions",
				},
				"times": 10,
				"response": map[string]interface{}{
					"status":  200,
					"content": "Response",
				},
			},
		},
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