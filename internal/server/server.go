package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fabianvf/llemulator/internal/models"
	"github.com/fabianvf/llemulator/internal/script"
)

type Server struct {
	engine *script.Engine
	debug  bool
}

func NewServer() *Server {
	return &Server{
		engine: script.NewEngine(),
		debug:  os.Getenv("DEBUG") == "true",
	}
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	
	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	
	return parts[1]
}

func writeError(w http.ResponseWriter, status int, message, errorType string, param, code *string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	errResp := models.ErrorResponse{
		Error: models.ErrorDetail{
			Message: message,
			Type:    errorType,
			Param:   param,
			Code:    code,
		},
	}
	
	json.NewEncoder(w).Encode(errResp)
}

func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) HandleScript(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid authorization", "auth_error", nil, nil)
		return
	}
	
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error", nil, nil)
		return
	}
	
	var scriptReq script.Script
	if err := json.Unmarshal(body, &scriptReq); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", "invalid_request_error", nil, nil)
		return
	}
	
	if err := s.engine.LoadScript(token, scriptReq); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "server_error", nil, nil)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "loaded"})
}

func (s *Server) HandleReset(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid authorization", "auth_error", nil, nil)
		return
	}
	
	s.engine.ResetSession(token)
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func (s *Server) HandleState(w http.ResponseWriter, r *http.Request) {
	if !s.debug {
		writeError(w, http.StatusForbidden, "Debug mode not enabled", "forbidden", nil, nil)
		return
	}
	
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid authorization", "auth_error", nil, nil)
		return
	}
	
	state, err := s.engine.GetState(token)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), "not_found", nil, nil)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

func (s *Server) HandleOpenAIRequest(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid authorization", "auth_error", nil, nil)
		return
	}
	
	body, err := readRequestBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error", nil, nil)
		return
	}
	
	s.logDebug(r, token, body)
	
	response, err := s.engine.MatchRequest(token, r.Method, r.URL.Path, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("No matching rule: %v", err), "invalid_request_error", nil, nil)
		return
	}
	
	s.writeResponse(w, r.URL.Path, body, response)
}

func readRequestBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func (s *Server) logDebug(r *http.Request, token string, body []byte) {
	if !s.debug {
		return
	}
	fmt.Printf("[DEBUG] Request: %s %s (token: %s)\n", r.Method, r.URL.Path, token)
	if len(body) > 0 {
		fmt.Printf("[DEBUG] Body: %s\n", string(body))
	}
}

func (s *Server) writeResponse(w http.ResponseWriter, path string, requestBody []byte, response *script.ResponseRule) {
	// If content is provided, auto-wrap it based on endpoint
	if response.Content != "" {
		s.writeWrappedResponse(w, path, requestBody, response)
		return
	}
	
	// Use explicit SSE if provided
	if len(response.SSE) > 0 {
		s.handleSSEResponse(w, response.SSE)
		return
	}
	
	// Use explicit JSON if provided
	if len(response.JSON) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.Status)
		w.Write(response.JSON)
		return
	}
	
	w.WriteHeader(response.Status)
}

// handleSSEResponse writes SSE events with proper formatting
func (s *Server) handleSSEResponse(w http.ResponseWriter, events []script.SSEEvent) {
	setSSEHeaders(w)
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	
	for _, event := range events {
		writeSSEEvent(w, event)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond) // Simulate streaming delay
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}

func writeSSEEvent(w http.ResponseWriter, event script.SSEEvent) {
	var data string
	if err := json.Unmarshal(event.Data, &data); err == nil && data == "[DONE]" {
		fmt.Fprintf(w, "data: [DONE]\n\n")
	} else {
		fmt.Fprintf(w, "data: %s\n\n", event.Data)
	}
}

func generateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func (s *Server) writeWrappedResponse(w http.ResponseWriter, path string, requestBody []byte, response *script.ResponseRule) {
	// Parse request to check if streaming is requested
	var req map[string]interface{}
	json.Unmarshal(requestBody, &req)
	isStreaming := false
	if stream, ok := req["stream"].(bool); ok {
		isStreaming = stream
	}
	
	// Determine response format based on endpoint
	if strings.HasPrefix(path, "/v1/chat/completions") {
		if isStreaming {
			s.writeChatCompletionStream(w, response.Content, req)
		} else {
			s.writeChatCompletion(w, response.Content, req, response.Status)
		}
	} else if strings.HasPrefix(path, "/v1/responses") || strings.HasPrefix(path, "/v1/completions") {
		if isStreaming {
			s.writeCompletionStream(w, response.Content, req)
		} else {
			s.writeCompletion(w, response.Content, req, response.Status)
		}
	} else if strings.HasPrefix(path, "/v1/models") {
		// For models endpoint, content is the model ID
		s.writeModel(w, response.Content, response.Status)
	} else {
		// Default: return content as-is
		w.WriteHeader(response.Status)
		w.Write([]byte(response.Content))
	}
}

func (s *Server) writeChatCompletion(w http.ResponseWriter, content string, req map[string]interface{}, status int) {
	model := "gpt-4"
	if m, ok := req["model"].(string); ok {
		model = m
	}
	
	completion := models.ChatCompletion{
		ID:      generateID("chatcmpl"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ChatChoice{
			{
				Index: 0,
				Message: &models.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: stringPtr("stop"),
			},
		},
		Usage: &models.Usage{
			PromptTokens:     10,
			CompletionTokens: len(content) / 4, // Rough estimate
			TotalTokens:      10 + len(content)/4,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(completion)
}

func (s *Server) writeChatCompletionStream(w http.ResponseWriter, content string, req map[string]interface{}) {
	model := "gpt-4"
	if m, ok := req["model"].(string); ok {
		model = m
	}
	
	setSSEHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	
	id := generateID("chatcmpl")
	timestamp := time.Now().Unix()
	
	// Send initial chunk with role
	chunk := models.ChatCompletion{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: timestamp,
		Model:   model,
		Choices: []models.ChatChoice{
			{
				Index: 0,
				Delta: &models.ChatMessage{
					Role: "assistant",
				},
			},
		},
	}
	
	data, _ := json.Marshal(chunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
	time.Sleep(10 * time.Millisecond)
	
	// Send content in chunks (simulate streaming)
	words := strings.Fields(content)
	for i, word := range words {
		chunk := models.ChatCompletion{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: timestamp,
			Model:   model,
			Choices: []models.ChatChoice{
				{
					Index: 0,
					Delta: &models.ChatMessage{
						Content: word,
					},
				},
			},
		}
		
		if i < len(words)-1 {
			chunk.Choices[0].Delta.Content += " "
		}
		
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Send final chunk
	finalChunk := models.ChatCompletion{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: timestamp,
		Model:   model,
		Choices: []models.ChatChoice{
			{
				Index:        0,
				Delta:        &models.ChatMessage{},
				FinishReason: stringPtr("stop"),
			},
		},
	}
	
	data, _ = json.Marshal(finalChunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) writeCompletion(w http.ResponseWriter, content string, req map[string]interface{}, status int) {
	model := "gpt-4"
	if m, ok := req["model"].(string); ok {
		model = m
	}
	
	completion := models.TextCompletion{
		ID:      generateID("cmpl"),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ResponseChoice{
			{
				Text:         content,
				Index:        0,
				Logprobs:     nil,
				FinishReason: "stop",
			},
		},
		Usage: &models.Usage{
			PromptTokens:     5,
			CompletionTokens: len(content) / 4,
			TotalTokens:      5 + len(content)/4,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(completion)
}

func (s *Server) writeCompletionStream(w http.ResponseWriter, content string, req map[string]interface{}) {
	model := "gpt-4"
	if m, ok := req["model"].(string); ok {
		model = m
	}
	
	setSSEHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	
	id := generateID("cmpl")
	timestamp := time.Now().Unix()
	
	// Send content in chunks
	words := strings.Fields(content)
	for i, word := range words {
		chunk := models.TextCompletion{
			ID:      id,
			Object:  "text_completion",
			Created: timestamp,
			Model:   model,
			Choices: []models.ResponseChoice{
				{
					Text:  word,
					Index: 0,
				},
			},
		}
		
		if i < len(words)-1 {
			chunk.Choices[0].Text += " "
		}
		
		if i == len(words)-1 {
			chunk.Choices[0].FinishReason = "stop"
		}
		
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
	}
	
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) writeModel(w http.ResponseWriter, modelID string, status int) {
	if modelID == "" {
		modelID = "gpt-4"
	}
	
	model := models.Model{
		ID:      modelID,
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: "openai",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model)
}

func stringPtr(s string) *string {
	return &s
}

func (s *Server) Run(port string) error {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/healthz", s.HandleHealthz)
	mux.HandleFunc("/readyz", s.HandleReadyz)
	
	mux.HandleFunc("POST /_emulator/script", s.HandleScript)
	mux.HandleFunc("POST /_emulator/reset", s.HandleReset)
	mux.HandleFunc("GET /_emulator/state", s.HandleState)
	
	mux.HandleFunc("/v1/", s.HandleOpenAIRequest)
	
	fmt.Printf("Starting server on port %s\n", port)
	return http.ListenAndServe(":"+port, mux)
}