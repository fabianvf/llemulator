package server

import (
	"strings"
	"testing"
	"net/http/httptest"
	"encoding/json"
	"bytes"
	"net/http"
	"io"
	
	"github.com/fabianvf/llemulator/internal/script"
	"github.com/fabianvf/llemulator/internal/models"
)

// TestStreamingChatCompletion tests the streaming chat completion functionality
func TestStreamingChatCompletion(t *testing.T) {
	server := NewServer()
	
	// Load a script with a response
	token := "test-token"
	testScript := script.Script{
		Reset:     true,
		Responses: "This is a test response for streaming",
	}
	
	if err := server.engine.LoadScript(token, testScript); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Create a streaming request
	payload := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Test message"},
		},
		"stream": true,
	}
	
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.HandleOpenAIRequest(recorder, req)
	
	// Check response
	response := recorder.Result()
	
	// Verify SSE headers
	if ct := response.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}
	
	// Read and verify SSE events
	bodyBytes, _ := io.ReadAll(response.Body)
	bodyStr := string(bodyBytes)
	
	// Should contain data: lines
	if !strings.Contains(bodyStr, "data: ") {
		t.Error("Response should contain SSE data events")
	}
	
	// Should end with [DONE]
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Error("Stream should end with [DONE]")
	}
	
	// Parse one of the events to verify structure
	lines := strings.Split(bodyStr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			dataStr := strings.TrimPrefix(line, "data: ")
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
				t.Errorf("Failed to parse chunk: %v", err)
			}
			// Verify chunk has expected structure
			if object, ok := chunk["object"].(string); !ok || object != "chat.completion.chunk" {
				t.Errorf("Expected object 'chat.completion.chunk', got %v", chunk["object"])
			}
			break
		}
	}
}

// TestStreamingCompletion tests the streaming completion functionality
func TestStreamingCompletion(t *testing.T) {
	server := NewServer()
	
	// Load a script with a response
	token := "test-token"
	testScript := script.Script{
		Reset:     true,
		Responses: "This is a test completion response",
	}
	
	if err := server.engine.LoadScript(token, testScript); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Create a streaming request
	payload := map[string]interface{}{
		"model": "gpt-4",
		"prompt": "Complete this",
		"stream": true,
	}
	
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.HandleOpenAIRequest(recorder, req)
	
	// Check response
	response := recorder.Result()
	
	// Verify SSE headers
	if ct := response.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}
	
	// Read and verify SSE events
	bodyBytes, _ := io.ReadAll(response.Body)
	bodyStr := string(bodyBytes)
	
	// Should contain data: lines
	if !strings.Contains(bodyStr, "data: ") {
		t.Error("Response should contain SSE data events")
	}
	
	// Should end with [DONE]
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Error("Stream should end with [DONE]")
	}
	
	// Parse one of the events to verify structure
	lines := strings.Split(bodyStr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			dataStr := strings.TrimPrefix(line, "data: ")
			var completion map[string]interface{}
			if err := json.Unmarshal([]byte(dataStr), &completion); err != nil {
				t.Errorf("Failed to parse completion: %v", err)
			}
			// Verify completion has expected structure
			if object, ok := completion["object"].(string); !ok || object != "text_completion" {
				t.Errorf("Expected object 'text_completion', got %v", completion["object"])
			}
			break
		}
	}
}

// TestNonStreamingChatCompletion tests non-streaming chat completion
func TestNonStreamingChatCompletion(t *testing.T) {
	server := NewServer()
	
	// Load a script with a response
	token := "test-token"
	testScript := script.Script{
		Reset:     true,
		Responses: "This is a non-streaming response",
	}
	
	if err := server.engine.LoadScript(token, testScript); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}
	
	// Create a non-streaming request
	payload := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Test message"},
		},
		"stream": false,
	}
	
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	server.HandleOpenAIRequest(recorder, req)
	
	// Check response
	response := recorder.Result()
	
	// Verify JSON content type
	if ct := response.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
	
	// Parse response
	var completion models.ChatCompletion
	if err := json.NewDecoder(response.Body).Decode(&completion); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	
	// Verify response structure
	if completion.Object != "chat.completion" {
		t.Errorf("Expected object 'chat.completion', got %s", completion.Object)
	}
	
	if len(completion.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(completion.Choices))
	}
	
	if completion.Choices[0].Message.Content != "This is a non-streaming response" {
		t.Errorf("Expected response content, got %s", completion.Choices[0].Message.Content)
	}
}

// TestEmptyRequestBody tests handling of empty request body
func TestEmptyRequestBody(t *testing.T) {
	server := NewServer()
	
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	
	recorder := httptest.NewRecorder()
	server.HandleOpenAIRequest(recorder, req)
	
	// Should get an error for no matching rule (empty message)
	response := recorder.Result()
	if response.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", response.StatusCode)
	}
	
	var errorResp models.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	
	if errorResp.Error.Type != "server_error" {
		t.Errorf("Expected error type 'server_error', got %s", errorResp.Error.Type)
	}
}

// TestMissingAuthentication tests missing auth header
func TestMissingAuthentication(t *testing.T) {
	server := NewServer()
	
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	// No auth header
	
	recorder := httptest.NewRecorder()
	server.HandleOpenAIRequest(recorder, req)
	
	// Should get an auth error
	response := recorder.Result()
	if response.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", response.StatusCode)
	}
	
	var errorResp models.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}
	
	if errorResp.Error.Type != "auth_error" {
		t.Errorf("Expected error type 'auth_error', got %s", errorResp.Error.Type)
	}
}