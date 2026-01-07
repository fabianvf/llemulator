package session

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewManager tests creation of a new session manager
func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

// TestSessionIsolation verifies that each token has independent state
func TestSessionIsolation(t *testing.T) {
	manager := NewManager()
	
	token1 := "token-1"
	token2 := "token-2"
	
	// Create sessions for different tokens
	session1 := manager.GetOrCreateSession(token1)
	session2 := manager.GetOrCreateSession(token2)
	
	if session1 == session2 {
		t.Error("Different tokens returned same session")
	}
	
	// Modify state in session1
	session1.SetData("key", "value1")
	
	// Verify session2 is not affected
	if session2.GetData("key") != nil {
		t.Error("Session isolation violated: data leaked between tokens")
	}
}

// TestConcurrentRequests verifies that requests serialize per token
func TestConcurrentRequests(t *testing.T) {
	manager := NewManager()
	token := "test-token"
	
	session := manager.GetOrCreateSession(token)
	
	// Track execution order
	var executions []int
	var mu sync.Mutex
	
	// Launch concurrent requests for same token
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Execute within session (should serialize)
			session.Execute(func() {
				// Simulate work
				time.Sleep(10 * time.Millisecond)
				
				mu.Lock()
				executions = append(executions, id)
				mu.Unlock()
			})
		}(i)
	}
	
	wg.Wait()
	
	// Verify all executions completed
	if len(executions) != 10 {
		t.Errorf("Expected 10 executions, got %d", len(executions))
	}
}

// TestSessionReset verifies that reset clears all state for a token
func TestSessionReset(t *testing.T) {
	manager := NewManager()
	token := "reset-token"
	
	// Create session and add data
	session := manager.GetOrCreateSession(token)
	session.SetData("key1", "value1")
	session.SetData("key2", "value2")
	
	// Verify data exists
	if session.GetData("key1") != "value1" {
		t.Error("Failed to set initial data")
	}
	
	// Reset session
	manager.ResetSession(token)
	
	// Get session again (should be new/empty)
	newSession := manager.GetOrCreateSession(token)
	
	// Verify data is cleared
	if newSession.GetData("key1") != nil {
		t.Error("Session data not cleared after reset")
	}
	if newSession.GetData("key2") != nil {
		t.Error("Session data not cleared after reset")
	}
}

// TestMultipleTokensConcurrent verifies different tokens don't interfere
func TestMultipleTokensConcurrent(t *testing.T) {
	manager := NewManager()
	
	// Use multiple tokens concurrently
	tokens := []string{"token1", "token2", "token3", "token4", "token5"}
	var wg sync.WaitGroup
	
	for _, token := range tokens {
		wg.Add(1)
		go func(tk string) {
			defer wg.Done()
			
			session := manager.GetOrCreateSession(tk)
			
			// Each token sets its own data
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key-%d", i)
				value := fmt.Sprintf("%s-value-%d", tk, i)
				session.SetData(key, value)
				
				// Verify immediately
				if got := session.GetData(key); got != value {
					t.Errorf("Token %s: expected %s, got %v", tk, value, got)
				}
			}
		}(token)
	}
	
	wg.Wait()
	
	// Verify each token has its own data
	for _, token := range tokens {
		session := manager.GetOrCreateSession(token)
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%d", i)
			expected := fmt.Sprintf("%s-value-%d", token, i)
			if got := session.GetData(key); got != expected {
				t.Errorf("Token %s: data corruption - expected %s, got %v", token, expected, got)
			}
		}
	}
}

// TestSessionCleanup tests that sessions can be manually cleaned up
func TestSessionCleanup(t *testing.T) {
	manager := NewManager()
	
	// Create multiple sessions
	tokens := []string{"token1", "token2", "token3"}
	for _, token := range tokens {
		session := manager.GetOrCreateSession(token)
		session.SetData("key", "value")
	}
	
	// Verify all sessions exist
	for _, token := range tokens {
		if manager.GetSession(token) == nil {
			t.Errorf("Session %s should exist", token)
		}
	}
	
	// Reset one session
	manager.ResetSession("token2")
	
	// Verify token2 is gone but others remain
	if manager.GetSession("token2") != nil {
		t.Error("Reset session should be removed")
	}
	if manager.GetSession("token1") == nil || manager.GetSession("token3") == nil {
		t.Error("Other sessions should still exist")
	}
}

// TestSessionPersistence verifies sessions persist between requests
func TestSessionPersistence(t *testing.T) {
	manager := NewManager()
	token := "persist-token"
	
	// First request - set data
	session1 := manager.GetOrCreateSession(token)
	session1.SetData("counter", 1)
	session1.SetData("name", "test")
	
	// Second request - get same session
	session2 := manager.GetOrCreateSession(token)
	
	// Should be same session
	if session1 != session2 {
		t.Error("GetOrCreateSession should return existing session")
	}
	
	// Data should persist
	if session2.GetData("counter") != 1 {
		t.Error("Session data not persisted")
	}
	if session2.GetData("name") != "test" {
		t.Error("Session data not persisted")
	}
}

// TestRaceConditions tests for race conditions in concurrent access
func TestRaceConditions(t *testing.T) {
	manager := NewManager()
	
	// Run with -race flag to detect races
	var wg sync.WaitGroup
	
	// Concurrent session creation
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			token := fmt.Sprintf("token-%d", id%10) // Reuse some tokens
			session := manager.GetOrCreateSession(token)
			session.SetData("test", id)
		}(i)
	}
	
	// Concurrent resets
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			token := fmt.Sprintf("token-%d", id%10)
			manager.ResetSession(token)
		}(i)
	}
	
	wg.Wait()
}