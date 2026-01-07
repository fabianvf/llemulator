package script

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

// Rule represents a simple matching rule
type Rule struct {
	Pattern  string `json:"pattern,omitempty"`  // Optional regex pattern
	Response string `json:"response"`           // Response content
	Times    int    `json:"times,omitempty"`    // How many times to match (-1 = unlimited, 0 = exhausted)
}

// Script represents a script configuration
type Script struct {
	Reset     bool        `json:"reset"`
	Rules     []Rule      `json:"rules,omitempty"`     // Explicit rules
	Responses interface{} `json:"responses,omitempty"` // Simplified format (array or map)
	Models    []string    `json:"models,omitempty"`    // Custom model list
}

// Engine handles script execution with minimal complexity
type Engine struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// Session holds rules for a token
type Session struct {
	mu     sync.Mutex
	rules  []Rule
	models []string // Custom models for this session
}

// NewEngine creates a new engine
func NewEngine() *Engine {
	return &Engine{
		sessions: make(map[string]*Session),
	}
}

// LoadScript loads a script for a token
func (e *Engine) LoadScript(token string, script Script) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var rules []Rule
	
	// Process simplified response format
	if script.Responses != nil {
		processedRules, err := processResponses(script.Responses)
		if err != nil {
			return err
		}
		rules = append(rules, processedRules...)
	}
	
	// Add explicit rules
	rules = append(rules, script.Rules...)
	
	// Create or reset session
	session, exists := e.sessions[token]
	if !exists || script.Reset {
		session = &Session{
			rules:  rules,
			models: script.Models,
		}
		e.sessions[token] = session
	} else {
		// Append rules without reset
		session.rules = append(session.rules, rules...)
		if len(script.Models) > 0 {
			session.models = script.Models
		}
	}
	
	return nil
}

// MatchRequest finds a response for a request
func (e *Engine) MatchRequest(token string, message string) (string, error) {
	e.mu.RLock()
	session, exists := e.sessions[token]
	e.mu.RUnlock()
	
	if !exists {
		return "", fmt.Errorf("no script loaded for token")
	}
	
	session.mu.Lock()
	defer session.mu.Unlock()
	
	// Try to match against rules
	for i, rule := range session.rules {
		// Skip exhausted rules
		if rule.Times == 0 {
			continue
		}
		
		// Check if pattern matches (or no pattern = always match)
		if rule.Pattern == "" || matchesPattern(rule.Pattern, message) {
			// Decrement counter unless unlimited
			if session.rules[i].Times > 0 {
				session.rules[i].Times--
			}
			return rule.Response, nil
		}
	}
	
	return "", fmt.Errorf("no matching rule for message: %s", message)
}

// Reset clears session for a token
func (e *Engine) Reset(token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.sessions, token)
}

// GetModels returns the list of valid models for a token
func (e *Engine) GetModels(token string) []string {
	// Check if session has custom models
	e.mu.RLock()
	session, exists := e.sessions[token]
	e.mu.RUnlock()
	
	if exists && len(session.models) > 0 {
		session.mu.Lock()
		defer session.mu.Unlock()
		return session.models
	}
	
	// Return default models
	return []string{
		"gpt-4", "gpt-4-turbo", "gpt-4-turbo-preview",
		"gpt-3.5-turbo", "gpt-3.5-turbo-16k",
		"text-davinci-003", "text-davinci-002",
		"text-embedding-ada-002",
	}
}

// ValidateModel checks if a model is valid for the given token
func (e *Engine) ValidateModel(token string, model string) bool {
	models := e.GetModels(token)
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// processResponses converts simplified formats to rules
func processResponses(responses interface{}) ([]Rule, error) {
	var rules []Rule
	
	switch v := responses.(type) {
	case []interface{}:
		// Sequential responses: ["response1", "response2", ...]
		for _, item := range v {
			if str, ok := item.(string); ok {
				rules = append(rules, Rule{
					Response: str,
					Times:    1,
				})
			} else if ruleMap, ok := item.(map[string]interface{}); ok {
				// Mixed mode: can have patterns in array
				if pattern, hasPattern := ruleMap["pattern"].(string); hasPattern {
					if response, hasResponse := ruleMap["response"].(string); hasResponse {
						times := 1
						if t, hasT := ruleMap["times"].(float64); hasT {
							times = int(t)
						}
						rules = append(rules, Rule{
							Pattern:  pattern,
							Response: response,
							Times:    times,
						})
					}
				}
			}
		}
		
	case map[string]interface{}:
		// Pattern-based: {"pattern": "response", ...}
		for pattern, response := range v {
			if respStr, ok := response.(string); ok {
				rules = append(rules, Rule{
					Pattern:  pattern,
					Response: respStr,
					Times:    -1, // Unlimited for pattern-based
				})
			}
		}
		
	case string:
		// Single response
		rules = append(rules, Rule{
			Response: v,
			Times:    1,
		})
		
	default:
		return nil, fmt.Errorf("unsupported response format")
	}
	
	return rules, nil
}

// matchesPattern checks if text matches a regex pattern
func matchesPattern(pattern, text string) bool {
	// Try to compile as regex
	re, err := regexp.Compile(pattern)
	if err != nil {
		// If not valid regex, do exact match
		return pattern == text
	}
	return re.MatchString(text)
}

// ExtractUserMessage extracts the user message from various request formats
func ExtractUserMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	
	// Try chat completion format
	if messages, ok := data["messages"].([]interface{}); ok {
		// Find last user message
		for i := len(messages) - 1; i >= 0; i-- {
			if msg, ok := messages[i].(map[string]interface{}); ok {
				if role, ok := msg["role"].(string); ok && role == "user" {
					if content, ok := msg["content"].(string); ok {
						return content
					}
				}
			}
		}
	}
	
	// Try completion format
	if prompt, ok := data["prompt"].(string); ok {
		return prompt
	}
	
	// Try direct input field
	if input, ok := data["input"].(string); ok {
		return input
	}
	
	return ""
}