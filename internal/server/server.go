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
	
	s.engine.Reset(token)
	
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
	
	// Simple debug response - we don't maintain detailed state anymore
	debugInfo := map[string]interface{}{
		"token": token,
		"status": "active",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugInfo)
}

func (s *Server) HandleOpenAIRequest(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Missing or invalid authorization", "auth_error", nil, nil)
		return
	}
	
	// Handle models endpoints separately (they are GET requests without body)
	if strings.Contains(r.URL.Path, "/models") {
		s.writeModelResponse(w, r.URL.Path, token)
		return
	}
	
	body, err := readRequestBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request_error", nil, nil)
		return
	}
	
	s.logDebug(r, token, body)
	
	// Validate JSON and model for non-GET requests
	if r.Method != "GET" && len(body) > 0 {
		var requestData map[string]interface{}
		if err := json.Unmarshal(body, &requestData); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON", "invalid_request_error", nil, nil)
			return
		}
		
		// Validate model if present
		if model, hasModel := requestData["model"].(string); hasModel {
			if !s.engine.ValidateModel(token, model) {
				modelParam := "model"
				writeError(w, http.StatusNotFound, fmt.Sprintf("The model `%s` does not exist", model), "invalid_request_error", &modelParam, nil)
				return
			}
		}
	}
	
	// Extract user message from request
	message := script.ExtractUserMessage(body)
	
	// Get response content from engine
	responseContent, err := s.engine.MatchRequest(token, message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("No matching rule: %v", err), "server_error", nil, nil)
		return
	}
	
	// Write the response in appropriate format for the endpoint
	s.writeFormattedResponse(w, r.URL.Path, body, responseContent)
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

func (s *Server) writeFormattedResponse(w http.ResponseWriter, path string, requestBody []byte, content string) {
	// Parse request to check if streaming is requested
	var req map[string]interface{}
	json.Unmarshal(requestBody, &req)
	isStreaming := false
	if stream, ok := req["stream"].(bool); ok {
		isStreaming = stream
	}
	
	// Format response based on endpoint
	if strings.Contains(path, "/chat/completions") {
		if isStreaming {
			s.writeChatCompletionStream(w, content, req)
		} else {
			s.writeChatCompletion(w, content, req, http.StatusOK)
		}
	} else if strings.Contains(path, "/completions") || strings.Contains(path, "/responses") {
		if isStreaming {
			s.writeCompletionStream(w, content, req)
		} else {
			s.writeCompletion(w, content, req, http.StatusOK)
		}
	} else {
		// Default: return as plain text
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
}


func generateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func (s *Server) writeModelResponse(w http.ResponseWriter, path string, token string) {
	// Get valid models for this token
	validModels := s.engine.GetModels(token)
	
	// Handle specific model retrieval
	if strings.Contains(path, "/models/") && !strings.HasSuffix(path, "/models") {
		// Extract model ID from path
		parts := strings.Split(path, "/")
		modelID := parts[len(parts)-1]
		
		// Check if model is valid
		isValid := false
		for _, m := range validModels {
			if m == modelID {
				isValid = true
				break
			}
		}
		
		if !isValid {
			modelParam := "model"
			writeError(w, http.StatusNotFound, fmt.Sprintf("The model `%s` does not exist", modelID), "invalid_request_error", &modelParam, nil)
			return
		}
		
		model := models.Model{
			ID:      modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "openai",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(model)
	} else {
		// List all models
		var modelList []models.Model
		for _, modelID := range validModels {
			modelList = append(modelList, models.Model{
				ID:      modelID,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "openai",
			})
		}
		
		list := models.ModelList{
			Object: "list",
			Data:   modelList,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(list)
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
			CompletionTokens: len(content) / 4,
			TotalTokens:      10 + len(content)/4,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(completion)
}

func stringPtr(s string) *string {
	return &s
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
	
	// Send initial chunk with role
	chunk := models.ChatCompletion{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
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
	
	// Send content in chunks
	words := strings.Fields(content)
	for i, word := range words {
		chunk := models.ChatCompletion{
			ID:      id,
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
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
	
	// Send finish chunk
	finishChunk := models.ChatCompletion{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ChatChoice{
			{
				Index:        0,
				Delta:        &models.ChatMessage{},
				FinishReason: stringPtr("stop"),
			},
		},
	}
	
	data, _ = json.Marshal(finishChunk)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
	
	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (s *Server) writeCompletion(w http.ResponseWriter, content string, req map[string]interface{}, status int) {
	model := "gpt-3.5-turbo-instruct"
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
				FinishReason: "stop",
			},
		},
		Usage: &models.Usage{
			PromptTokens:     10,
			CompletionTokens: len(content) / 4,
			TotalTokens:      10 + len(content)/4,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(completion)
}

func (s *Server) writeCompletionStream(w http.ResponseWriter, content string, req map[string]interface{}) {
	model := "gpt-3.5-turbo-instruct"
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
	
	// Send content in chunks
	words := strings.Fields(content)
	for i, word := range words {
		chunk := models.TextCompletion{
			ID:      id,
			Object:  "text_completion",
			Created: time.Now().Unix(),
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
		
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Send [DONE]
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// setupRoutes creates the router with all handlers
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/healthz", s.HandleHealthz)
	mux.HandleFunc("/readyz", s.HandleReadyz)
	
	mux.HandleFunc("POST /_emulator/script", s.HandleScript)
	mux.HandleFunc("POST /_emulator/reset", s.HandleReset)
	mux.HandleFunc("GET /_emulator/state", s.HandleState)
	
	mux.HandleFunc("/v1/", s.HandleOpenAIRequest)
	
	return mux
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setupRoutes().ServeHTTP(w, r)
}

func (s *Server) Run(port string) error {
	fmt.Printf("Starting server on port %s\n", port)
	return http.ListenAndServe(":"+port, s.setupRoutes())
}