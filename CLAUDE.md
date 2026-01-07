# OpenAI API Emulator - Development Guide

## Overview
This is an OpenAI API emulator designed for testing and CI/CD pipelines. It provides deterministic, scriptable responses that make real OpenAI client libraries "just work" without actual API calls.

## Key Architecture Decisions

### Language: Go
- Chosen for excellent concurrency support (streaming SSE)
- Native Kubernetes deployment compatibility
- Fast startup and low memory footprint
- Easy to build single binary deployments

### Core Components

1. **Script Engine** (`internal/script/`)
   - Request matching based on method, path, and JSON fields
   - Rule-based response selection with counter support
   - Per-token isolation for concurrent test execution

2. **Session Management** (`internal/session/`)
   - Token-scoped state isolation
   - Per-token mutex to prevent request interleaving
   - Script rule storage and counter management

3. **Server** (`internal/server/`)
   - HTTP routing for OpenAI API endpoints
   - SSE streaming implementation for responses and chat
   - Middleware for token extraction and session handling

4. **Models** (`internal/models/`)
   - OpenAI API response types and schemas
   - Error envelope structures
   - Streaming event types

## Running the System

### Local Development
```bash
# Run the server
go run cmd/openai-emulator/main.go

# Run all tests (Go unit tests + conformance)
make test

# Run only conformance tests
make conformance

# Run specific conformance suite
cd conformance/js && npm test
cd conformance/python && pytest
```

### Docker
```bash
# Build image
docker build -t openai-emulator .

# Run container
docker run -p 8080:8080 openai-emulator

# Run with environment variables
docker run -p 8080:8080 -e DEBUG=true openai-emulator
```

### Kubernetes
```bash
# Deploy using manifests
kubectl apply -f k8s/

# Or use Helm (if chart provided)
helm install openai-emulator ./charts/openai-emulator
```

## API Endpoints

### OpenAI Compatible Endpoints (P0)
- `GET /v1/models` - List available models
- `GET /v1/models/{id}` - Get specific model
- `POST /v1/responses` - Create response (streaming/non-streaming)
- `POST /v1/chat/completions` - Chat completions (streaming/non-streaming)

### Admin Control Plane
- `POST /_emulator/script` - Load script rules for token
- `POST /_emulator/reset` - Clear session state
- `GET /_emulator/state` - Debug endpoint (when DEBUG=true)

### Health Checks
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe

## Script Format

Scripts are loaded via `POST /_emulator/script` with Bearer token auth:

```json
{
  "reset": true,
  "rules": [
    {
      "match": {
        "method": "POST",
        "path": "/v1/chat/completions",
        "json": {"model": "gpt-4"}
      },
      "times": 1,
      "response": {
        "status": 200,
        "json": {
          "id": "chatcmpl-123",
          "object": "chat.completion",
          "model": "gpt-4",
          "choices": [{
            "index": 0,
            "message": {
              "role": "assistant",
              "content": "Hello!"
            },
            "finish_reason": "stop"
          }]
        }
      }
    },
    {
      "match": {
        "method": "POST",
        "path": "/v1/chat/completions",
        "json": {"stream": true}
      },
      "times": 1,
      "response": {
        "status": 200,
        "sse": [
          {"data": {"id": "chatcmpl-123", "object": "chat.completion.chunk", "model": "gpt-4", "choices": [{"index": 0, "delta": {"role": "assistant"}, "finish_reason": null}]}},
          {"data": {"id": "chatcmpl-123", "object": "chat.completion.chunk", "model": "gpt-4", "choices": [{"index": 0, "delta": {"content": "Hello"}, "finish_reason": null}]}},
          {"data": {"id": "chatcmpl-123", "object": "chat.completion.chunk", "model": "gpt-4", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]}},
          {"data": "[DONE]"}
        ]
      }
    }
  ],
  "defaults": {
    "on_unmatched": "error"
  }
}
```

## Testing Strategy

### Conformance Tests (MUST PASS)
Located in `conformance/` - these are the contract:
- JavaScript SDK compatibility tests
- Python SDK compatibility tests
- Both test streaming and non-streaming
- Must use official OpenAI SDKs

### Running Tests
```bash
# All tests
make test

# Go unit tests only
go test ./...

# JS conformance only
cd conformance/js && npm test

# Python conformance only
cd conformance/python && pytest

# With coverage
go test -cover ./...
```

## Common Development Tasks

### Adding a New Endpoint
1. Define response types in `internal/models/`
2. Add route in `internal/server/routes.go`
3. Implement handler in `internal/server/handlers.go`
4. Add conformance test in both JS and Python suites
5. Update script matching if needed

### Debugging Streaming Issues
- Check SSE event format: `data: <json>\n\n`
- Verify flush after each event
- Ensure proper stream termination
- Use curl for raw SSE testing:
  ```bash
  curl -N -H "Authorization: Bearer test-token" \
       -H "Content-Type: application/json" \
       -d '{"model":"gpt-4","stream":true,"messages":[{"role":"user","content":"Hi"}]}' \
       http://localhost:8080/v1/chat/completions
  ```

### Session Debugging
When `DEBUG=true`, use `GET /_emulator/state` to inspect:
- Remaining rule counters
- Last matched rules
- Current session state

## Important Implementation Notes

1. **Streaming Must Flush**: After writing each SSE event, call `Flush()` on the response writer
2. **Token Isolation**: Every token gets its own mutex - serialize requests per token
3. **Rule Matching**: First match wins, then decrement counter
4. **Error Format**: Must match OpenAI error envelope for SDK exception handling
5. **Model List Calls**: SDKs may call `/v1/models` automatically - handle extra calls gracefully

## Environment Variables

- `PORT` - Server port (default: 8080)
- `DEBUG` - Enable debug endpoints (default: false)
- `LOG_LEVEL` - Logging verbosity (debug/info/warn/error)

## Troubleshooting

### SDK Can't Connect
- Check base_url configuration in SDK client
- Verify server is running on expected port
- Check for proxy or firewall issues

### Streaming Not Working
- Verify Content-Type is `text/event-stream`
- Check that events are properly formatted
- Ensure response writer supports flushing
- Test with curl to isolate SDK issues

### Script Not Matching
- Enable DEBUG mode and check `/_emulator/state`
- Verify JSON subset matching logic
- Check rule ordering (first match wins)
- Ensure token is consistent across calls

## Performance Considerations

- Sessions are in-memory (not distributed)
- Each token has independent state
- Mutex per token prevents concurrent request processing
- Streaming responses hold connections open
- Consider memory limits for large scripts

## Security Notes

- Tokens are for test isolation, not authentication
- Admin endpoints should not be exposed in production
- Debug endpoints must be gated by environment variable
- No real API keys should be used with this emulator

## Future Enhancements (P1/P2)

- Embeddings API support
- Files API support  
- Batch API support
- Response caching
- Distributed session storage
- Metrics and observability
- OpenTelemetry integration