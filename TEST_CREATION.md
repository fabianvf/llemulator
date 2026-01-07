# Test Creation Guide for OpenAI Emulator

This guide explains how to create test scripts for the OpenAI API emulator.

## Quick Start

The emulator supports multiple formats for defining responses, from simple to advanced.

### Simplest Format - Sequential Responses

Just provide an array of responses that will be returned in order:

```javascript
{
  "responses": ["First response", "Second response", "Third response"]
}
```

Each API call will get the next response in the sequence.

### Pattern Matching

Use regex patterns to match specific user messages:

```javascript
{
  "responses": {
    "hello": "Hi there!",                    // Exact match
    ".*weather.*": "It's sunny today!",      // Regex pattern
    "\\d+\\s*\\+\\s*\\d+": "I can't do math!" // Complex pattern
  }
}
```

Patterns are case-insensitive regular expressions. The first matching pattern wins.

### Mixed Format

Combine sequential responses with pattern matching:

```javascript
{
  "responses": [
    "Default first response",
    { "match": "help", "response": "How can I help?" },
    { "match": "error", "error": "Something went wrong", "status": 500 }
  ]
}
```

## How It Works

### Automatic Response Wrapping

The emulator automatically wraps your content in the appropriate OpenAI format:

- **Chat Completions** (`/v1/chat/completions`): Wraps as ChatCompletion response
- **Text Completions** (`/v1/responses`): Wraps as TextCompletion response  
- **Models** (`/v1/models`): Wraps as Model response

### Streaming Support

The same response content works for both streaming and non-streaming requests. The emulator automatically handles streaming based on the `stream` parameter in the request:

```javascript
{
  "responses": ["This works for both streaming and non-streaming!"]
}
```

- If client sends `stream: false` → Returns complete JSON response
- If client sends `stream: true` → Returns Server-Sent Events (SSE) chunks

## Advanced Format

For full control, use the traditional rule-based format:

```javascript
{
  "rules": [
    {
      "match": {
        "method": "POST",
        "path": "/v1/chat/completions",
        "json": { "model": "gpt-4" }
      },
      "times": 1,
      "response": {
        "status": 200,
        "json": { /* Full OpenAI response format */ }
      }
    }
  ]
}
```

### Rule Matching

Rules are matched in order based on:
1. HTTP method
2. Path
3. JSON body content (subset matching)
4. Pattern (if specified)

### SSE Streaming

For explicit SSE control:

```javascript
{
  "rules": [
    {
      "match": { "method": "POST", "path": "/v1/chat/completions" },
      "response": {
        "status": 200,
        "sse": [
          { "data": { /* chunk 1 */ } },
          { "data": { /* chunk 2 */ } },
          { "data": "[DONE]" }
        ]
      }
    }
  ]
}
```

## Loading Scripts

### JavaScript/Node.js

```javascript
async function loadScript(config) {
  const response = await fetch('http://localhost:8080/_emulator/script', {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer your-token',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      reset: true,
      ...config,
      defaults: { on_unmatched: 'error' }
    }),
  });
  
  if (!response.ok) {
    throw new Error('Failed to load script');
  }
}

// Usage
await loadScript({
  responses: ['Hello!', 'How are you?', 'Goodbye!']
});
```

### Python

```python
import httpx

def load_script(config):
    response = httpx.post(
        'http://localhost:8080/_emulator/script',
        headers={'Authorization': 'Bearer your-token'},
        json={
            'reset': True,
            **config,
            'defaults': {'on_unmatched': 'error'}
        }
    )
    if response.status_code != 200:
        raise Exception(f'Failed to load script: {response.text}')

# Usage
load_script({
    'responses': ['Hello!', 'How are you?', 'Goodbye!']
})
```

## Examples

### Testing a Chatbot

```javascript
{
  "responses": {
    "hello|hi|hey": "Hello! How can I help you today?",
    ".*weather.*": "I can't check the weather, but it looks nice outside!",
    ".*joke.*": "Why did the test pass? Because it asserted itself!",
    "bye|goodbye": "Goodbye! Have a great day!"
  }
}
```

### Testing Sequential Interactions

```javascript
{
  "responses": [
    "Welcome! I'm your assistant.",
    "I can help you with various tasks.",
    "What would you like to know?",
    "That's interesting! Tell me more.",
    "I understand. Is there anything else?"
  ]
}
```

### Testing Error Handling

```javascript
{
  "responses": [
    "Normal response",
    { "match": "break", "error": "Something went wrong", "status": 500 },
    { "match": "unauthorized", "error": "Invalid API key", "status": 401 }
  ]
}
```

### Testing Different Models

```javascript
{
  "rules": [
    {
      "match": { 
        "method": "POST", 
        "path": "/v1/chat/completions",
        "json": { "model": "gpt-3.5-turbo" }
      },
      "response": {
        "status": 200,
        "content": "I'm GPT-3.5!"
      }
    },
    {
      "match": { 
        "method": "POST", 
        "path": "/v1/chat/completions",
        "json": { "model": "gpt-4" }
      },
      "response": {
        "status": 200,
        "content": "I'm GPT-4!"
      }
    }
  ]
}
```

## Admin Endpoints

### Load Script
```
POST /_emulator/script
Authorization: Bearer <token>
```

### Reset Session
```
POST /_emulator/reset
Authorization: Bearer <token>
```

### Get State (Debug Mode)
```
GET /_emulator/state
Authorization: Bearer <token>
```

## Best Practices

1. **Start Simple**: Use the array format for basic testing
2. **Use Patterns**: Regex patterns handle variations in user input
3. **Test Streaming**: The same content works for both modes
4. **Isolate Tests**: Use `reset: true` to clear state between test suites
5. **Token Isolation**: Each token has its own session state

## Troubleshooting

### No Matching Rule Found
- Check your patterns are valid regex
- Remember patterns are case-insensitive
- Verify the request path and method match

### Streaming Not Working
- Ensure client sends `stream: true` in request body
- Check that response content is provided (not empty)

### Pattern Not Matching
- Test your regex pattern separately
- Remember to escape special characters (`\\d`, `\\s`, etc.)
- Use `.*` for flexible matching

## Migration from Old Format

If you have existing tests using the full OpenAI response format, you can simplify them:

**Before:**
```javascript
{
  "rules": [{
    "match": { "method": "POST", "path": "/v1/chat/completions" },
    "response": {
      "status": 200,
      "json": {
        "id": "chatcmpl-123",
        "object": "chat.completion",
        "created": 1234567890,
        "model": "gpt-4",
        "choices": [{
          "index": 0,
          "message": {
            "role": "assistant",
            "content": "Hello!"
          },
          "finish_reason": "stop"
        }],
        "usage": { /* ... */ }
      }
    }
  }]
}
```

**After:**
```javascript
{
  "responses": ["Hello!"]
}
```

The emulator automatically generates all the OpenAI protocol details!