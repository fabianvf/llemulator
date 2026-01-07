package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fabianvf/llemulator/internal/models"
	"github.com/fabianvf/llemulator/internal/script"
)

// TestSSEFormat verifies correct event-stream format
func TestSSEFormat(t *testing.T) {
	server := NewServer()
	
	// Setup test response
	events := []script.SSEEvent{
		{Data: json.RawMessage(`{"test": "data1"}`)},
		{Data: json.RawMessage(`{"test": "data2"}`)},
		{Data: json.RawMessage(`"[DONE]"`)},
	}
	
	recorder := httptest.NewRecorder()
	server.streamSSEResponse(recorder, events)
	
	response := recorder.Result()
	
	// Check headers
	if ct := response.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := response.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", cc)
	}
	if conn := response.Header.Get("Connection"); conn != "keep-alive" {
		t.Errorf("Expected Connection keep-alive, got %s", conn)
	}
	
	// Parse SSE events
	body := recorder.Body.String()
	lines := strings.Split(body, "\n")
	
	// Verify SSE format: "data: <json>\n\n"
	eventCount := 0
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "data: ") {
			eventCount++
			// Should be followed by empty line
			if i+1 < len(lines) && lines[i+1] != "" {
				t.Errorf("SSE event not followed by empty line at line %d", i)
			}
		}
	}
	
	if eventCount != 3 {
		t.Errorf("Expected 3 SSE events, found %d", eventCount)
	}
	
	// Verify [DONE] event
	if !strings.Contains(body, "data: [DONE]") {
		t.Error("Missing [DONE] termination event")
	}
}

// TestStreamChunking verifies words are properly separated
func TestStreamChunking(t *testing.T) {
	server := NewServer()
	
	content := "This is a test message with multiple words"
	req := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
	}
	
	recorder := httptest.NewRecorder()
	server.writeChatCompletionStream(recorder, content, req)
	
	// Parse streamed chunks
	body := recorder.Body.String()
	lines := strings.Split(body, "\n")
	
	var chunks []string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			data := strings.TrimPrefix(line, "data: ")
			
			var chunk models.ChatCompletion
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // Skip non-JSON lines
			}
			
			if chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
				chunks = append(chunks, chunk.Choices[0].Delta.Content)
			}
		}
	}
	
	// Reconstruct message from chunks
	reconstructed := strings.Join(chunks, " ")
	
	// Should preserve the original message with proper spacing
	if !strings.Contains(reconstructed, "test message") {
		t.Errorf("Chunks not properly separated. Got: %s", reconstructed)
	}
	
	// Verify we got multiple chunks (not sent all at once)
	if len(chunks) < 3 {
		t.Errorf("Expected multiple chunks for streaming, got %d", len(chunks))
	}
}

// TestStreamCompletion verifies proper termination sequence
func TestStreamCompletion(t *testing.T) {
	server := NewServer()
	
	content := "Short message"
	req := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
	}
	
	recorder := httptest.NewRecorder()
	server.writeChatCompletionStream(recorder, content, req)
	
	body := recorder.Body.String()
	lines := strings.Split(body, "\n")
	
	// Track the sequence of events
	var hasRole bool
	var hasContent bool
	var hasFinishReason bool
	var hasDone bool
	
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			if data == "[DONE]" {
				hasDone = true
				continue
			}
			
			var chunk models.ChatCompletion
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			
			if chunk.Choices[0].Delta != nil {
				if chunk.Choices[0].Delta.Role == "assistant" {
					hasRole = true
				}
				if chunk.Choices[0].Delta.Content != "" {
					hasContent = true
				}
			}
			
			if chunk.Choices[0].FinishReason != nil && *chunk.Choices[0].FinishReason == "stop" {
				hasFinishReason = true
			}
		}
	}
	
	// Verify complete sequence
	if !hasRole {
		t.Error("Stream missing initial role chunk")
	}
	if !hasContent {
		t.Error("Stream missing content chunks")
	}
	if !hasFinishReason {
		t.Error("Stream missing finish_reason")
	}
	if !hasDone {
		t.Error("Stream missing [DONE] termination")
	}
	
	// Verify order: role -> content -> finish -> done
	roleIndex := strings.Index(body, `"role":"assistant"`)
	contentIndex := strings.Index(body, content)
	finishIndex := strings.Index(body, `"finish_reason":"stop"`)
	doneIndex := strings.Index(body, "[DONE]")
	
	if roleIndex > contentIndex || contentIndex > finishIndex || finishIndex > doneIndex {
		t.Error("Stream events in wrong order")
	}
}

// TestFlushBehavior verifies events flush immediately
func TestFlushBehavior(t *testing.T) {
	// This test simulates real streaming by checking timing
	server := NewServer()
	
	// Create a custom ResponseWriter that tracks flushes
	flushRecorder := &flushTracker{
		ResponseWriter: httptest.NewRecorder(),
		flushTimes:     []time.Time{},
	}
	
	events := []script.SSEEvent{
		{Data: json.RawMessage(`{"chunk": 1}`)},
		{Data: json.RawMessage(`{"chunk": 2}`)},
		{Data: json.RawMessage(`{"chunk": 3}`)},
	}
	
	server.streamSSEResponse(flushRecorder, events)
	
	// Should have flushed after each event
	if len(flushRecorder.flushTimes) != len(events) {
		t.Errorf("Expected %d flushes, got %d", len(events), len(flushRecorder.flushTimes))
	}
	
	// Verify flushes happen with delays (simulating streaming)
	for i := 1; i < len(flushRecorder.flushTimes); i++ {
		delay := flushRecorder.flushTimes[i].Sub(flushRecorder.flushTimes[i-1])
		if delay < 5*time.Millisecond {
			t.Error("Flushes happening too quickly, not simulating streaming")
		}
	}
}

// TestStreamErrors verifies error handling during stream
func TestStreamErrors(t *testing.T) {
	server := NewServer()
	
	testCases := []struct {
		name        string
		events      []script.SSEEvent
		expectError bool
	}{
		{
			name:        "Empty events",
			events:      []script.SSEEvent{},
			expectError: false, // Should handle gracefully
		},
		{
			name: "Malformed JSON in event",
			events: []script.SSEEvent{
				{Data: json.RawMessage(`{"valid": "json"}`)},
				{Data: json.RawMessage(`{invalid json`)},
				{Data: json.RawMessage(`"[DONE]"`)},
			},
			expectError: false, // Should continue streaming
		},
		{
			name: "Very large event",
			events: []script.SSEEvent{
				{Data: json.RawMessage(`{"data": "` + strings.Repeat("x", 1000000) + `"}`)},
			},
			expectError: false, // Should handle large events
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			
			// Should not panic
			func() {
				defer func() {
					if r := recover(); r != nil && !tc.expectError {
						t.Errorf("Unexpected panic: %v", r)
					}
				}()
				
				server.streamSSEResponse(recorder, tc.events)
			}()
			
			// Should still have proper headers
			response := recorder.Result()
			if ct := response.Header.Get("Content-Type"); ct != "text/event-stream" {
				t.Error("Lost Content-Type header after error")
			}
		})
	}
}

// TestNonStreamingFlusher tests behavior when ResponseWriter doesn't support flushing
func TestNonStreamingFlusher(t *testing.T) {
	server := NewServer()
	
	// Create a ResponseWriter that doesn't implement http.Flusher
	nonFlusher := &nonFlushingWriter{
		ResponseWriter: httptest.NewRecorder(),
	}
	
	events := []script.SSEEvent{
		{Data: json.RawMessage(`{"test": "data"}`)},
	}
	
	// Should handle gracefully without panicking
	server.streamSSEResponse(nonFlusher, events)
	
	// Should have written error response
	result := nonFlusher.ResponseWriter.(*httptest.ResponseRecorder).Result()
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected 500 error for non-flusher, got %d", result.StatusCode)
	}
}

// TestConcurrentStreaming tests concurrent streaming requests
func TestConcurrentStreaming(t *testing.T) {
	server := NewServer()
	
	// Run multiple streaming requests concurrently
	concurrency := 10
	done := make(chan bool, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			recorder := httptest.NewRecorder()
			
			content := fmt.Sprintf("Message %d", id)
			req := map[string]interface{}{
				"model":  "gpt-4",
				"stream": true,
			}
			
			server.writeChatCompletionStream(recorder, content, req)
			
			// Verify response contains expected content
			body := recorder.Body.String()
			if !strings.Contains(body, content) {
				t.Errorf("Stream %d missing content", id)
			}
			if !strings.Contains(body, "[DONE]") {
				t.Errorf("Stream %d missing termination", id)
			}
			
			done <- true
		}(i)
	}
	
	// Wait for all streams to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// Helper types for testing

type flushTracker struct {
	http.ResponseWriter
	flushTimes []time.Time
}

func (f *flushTracker) Flush() {
	f.flushTimes = append(f.flushTimes, time.Now())
	if flusher, ok := f.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type nonFlushingWriter struct {
	http.ResponseWriter
}

// TestStreamingWordBoundaries verifies words are complete, not partial
func TestStreamingWordBoundaries(t *testing.T) {
	server := NewServer()
	
	// Message with punctuation and special characters
	content := "Hello, world! This is a test-message with numbers: 123."
	req := map[string]interface{}{
		"model":  "gpt-4",
		"stream": true,
	}
	
	recorder := httptest.NewRecorder()
	server.writeChatCompletionStream(recorder, content, req)
	
	body := recorder.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	
	var words []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") && !strings.Contains(line, "[DONE]") {
			data := strings.TrimPrefix(line, "data: ")
			
			var chunk models.ChatCompletion
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != "" {
					words = append(words, chunk.Choices[0].Delta.Content)
				}
			}
		}
	}
	
	// Each chunk should be a complete word or punctuation
	for _, word := range words {
		// Should not have partial words
		if strings.HasPrefix(word, " ") || strings.HasSuffix(word, " ") {
			continue // Space handling is ok
		}
		
		// Check it's a complete token (word, number, or punctuation)
		if len(word) > 0 && !strings.ContainsAny(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!.,?-:") {
			t.Errorf("Chunk appears to be partial word: '%s'", word)
		}
	}
	
	// Reconstruct and verify message integrity
	reconstructed := strings.Join(words, " ")
	reconstructed = strings.ReplaceAll(reconstructed, " , ", ", ")
	reconstructed = strings.ReplaceAll(reconstructed, " ! ", "! ")
	reconstructed = strings.ReplaceAll(reconstructed, " . ", ". ")
	reconstructed = strings.ReplaceAll(reconstructed, " : ", ": ")
	
	// Should maintain the essence of the message
	if !strings.Contains(reconstructed, "Hello") || !strings.Contains(reconstructed, "world") {
		t.Errorf("Message corrupted during streaming: %s", reconstructed)
	}
}