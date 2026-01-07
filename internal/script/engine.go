package script

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

type MatchRule struct {
	Method  string          `json:"method"`
	Path    string          `json:"path"`
	JSON    json.RawMessage `json:"json,omitempty"`
	Pattern string          `json:"pattern,omitempty"`
}

type ResponseRule struct {
	Status  int             `json:"status"`
	Content string          `json:"content,omitempty"`
	JSON    json.RawMessage `json:"json,omitempty"`
	SSE     []SSEEvent      `json:"sse,omitempty"`
}

type SSEEvent struct {
	Data json.RawMessage `json:"data"`
}

type Rule struct {
	Match    MatchRule    `json:"match"`
	Times    int          `json:"times"`
	Response ResponseRule `json:"response"`
}

type Script struct {
	Reset     bool              `json:"reset"`
	Rules     []Rule            `json:"rules"`
	Responses interface{}       `json:"responses,omitempty"`
	Defaults  DefaultSettings   `json:"defaults"`
}

type DefaultSettings struct {
	OnUnmatched string `json:"on_unmatched"`
}

type Engine struct {
	mu       sync.RWMutex
	sessions map[string]*SessionState
}

type SessionState struct {
	mu    sync.Mutex
	rules []Rule
}

func NewEngine() *Engine {
	return &Engine{
		sessions: make(map[string]*SessionState),
	}
}

func (e *Engine) LoadScript(token string, script Script) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rules := script.Rules
	
	// Process simplified response format if provided
	if script.Responses != nil {
		processedRules, err := processResponses(script.Responses)
		if err != nil {
			return err
		}
		rules = append(rules, processedRules...)
	}

	session, exists := e.sessions[token]
	if !exists || script.Reset {
		session = &SessionState{
			rules: make([]Rule, len(rules)),
		}
		e.sessions[token] = session
	}

	for i, rule := range rules {
		session.rules[i] = rule
	}

	return nil
}

func (e *Engine) MatchRequest(token, method, path string, body []byte) (*ResponseRule, error) {
	session := e.getSession(token)
	if session == nil {
		return nil, fmt.Errorf("no script loaded for token")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	requestJSON := parseRequestBody(body)
	return findMatchingRule(session, method, path, requestJSON)
}

func (e *Engine) getSession(token string) *SessionState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessions[token]
}

func parseRequestBody(body []byte) map[string]interface{} {
	if len(body) == 0 {
		return nil
	}
	var requestJSON map[string]interface{}
	if err := json.Unmarshal(body, &requestJSON); err != nil {
		return nil
	}
	return requestJSON
}

func findMatchingRule(session *SessionState, method, path string, requestJSON map[string]interface{}) (*ResponseRule, error) {
	// Extract user message for pattern matching
	userMessage := extractUserMessage(requestJSON)
	
	for i, rule := range session.rules {
		if rule.Times <= 0 {
			continue
		}

		// Pattern matching takes precedence
		if rule.Match.Pattern != "" {
			if !matchesPattern(rule.Match.Pattern, userMessage) {
				continue
			}
		} else {
			// Traditional matching
			if !matchesMethodAndPath(rule, method, path) {
				continue
			}

			if !matchesJSON(rule, requestJSON) {
				continue
			}
		}

		session.rules[i].Times--
		return &rule.Response, nil
	}
	return nil, fmt.Errorf("no matching rule found")
}

func matchesMethodAndPath(rule Rule, method, path string) bool {
	return rule.Match.Method == method && rule.Match.Path == path
}

func matchesJSON(rule Rule, requestJSON map[string]interface{}) bool {
	if len(rule.Match.JSON) == 0 {
		return true
	}

	var matchJSON map[string]interface{}
	if err := json.Unmarshal(rule.Match.JSON, &matchJSON); err != nil {
		return false
	}

	if len(matchJSON) == 0 {
		return true
	}

	if requestJSON == nil {
		return false
	}

	return jsonContains(requestJSON, matchJSON)
}

// extractUserMessage extracts the user message from a chat completion request
func extractUserMessage(requestJSON map[string]interface{}) string {
	if requestJSON == nil {
		return ""
	}
	
	// Try to extract messages array
	if messages, ok := requestJSON["messages"].([]interface{}); ok {
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
	
	// Try to extract prompt for completion endpoint
	if prompt, ok := requestJSON["prompt"].(string); ok {
		return prompt
	}
	
	return ""
}

// matchesPattern checks if text matches a regex pattern (case-insensitive)
func matchesPattern(pattern, text string) bool {
	if pattern == "" || text == "" {
		return false
	}
	
	// Compile regex with case-insensitive flag
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		// If pattern is invalid regex, try exact match
		return strings.EqualFold(pattern, text)
	}
	
	return re.MatchString(text)
}

// processResponses converts simplified response formats to internal rules
func processResponses(responses interface{}) ([]Rule, error) {
	var rules []Rule
	
	switch v := responses.(type) {
	case []interface{}:
		// Sequential array format
		for _, resp := range v {
			rule := Rule{
				Match: MatchRule{
					Method: "POST",
					Path:   "/v1/chat/completions",
				},
				Times: 1,
				Response: ResponseRule{
					Status: 200,
				},
			}
			
			switch r := resp.(type) {
			case string:
				rule.Response.Content = r
			case map[string]interface{}:
				// Handle mixed format with match patterns
				if match, ok := r["match"].(string); ok {
					// This is a pattern match rule
					rule.Match.Pattern = match
					if response, ok := r["response"].(string); ok {
						rule.Response.Content = response
					}
					if err, ok := r["error"].(string); ok {
						rule.Response.Content = err
						if status, ok := r["status"].(float64); ok {
							rule.Response.Status = int(status)
						} else {
							rule.Response.Status = 500
						}
					}
				}
			}
			rules = append(rules, rule)
		}
	case map[string]interface{}:
		// Pattern matching format
		for pattern, response := range v {
			rule := Rule{
				Match: MatchRule{
					Method:  "POST",
					Path:    "/v1/chat/completions",
					Pattern: pattern,
				},
				Times: 999, // Pattern matches can be used many times
				Response: ResponseRule{
					Status:  200,
					Content: response.(string),
				},
			}
			rules = append(rules, rule)
		}
	}
	
	return rules, nil
}

func (e *Engine) ResetSession(token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.sessions, token)
}

func (e *Engine) GetState(token string) (map[string]interface{}, error) {
	e.mu.RLock()
	session, exists := e.sessions[token]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no session for token")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	state := map[string]interface{}{
		"rules": session.rules,
	}

	return state, nil
}

// jsonContains checks if target contains all fields from subset with deep matching
func jsonContains(target, subset map[string]interface{}) bool {
	for key, subValue := range subset {
		targetValue, exists := target[key]
		if !exists {
			return false
		}

		// Handle nested objects
		if subMap, ok := subValue.(map[string]interface{}); ok {
			if targetMap, ok := targetValue.(map[string]interface{}); ok {
				if !jsonContains(targetMap, subMap) {
					return false
				}
			} else {
				return false
			}
		} else {
			// Direct value comparison
			if !reflect.DeepEqual(targetValue, subValue) {
				return false
			}
		}
	}
	return true
}