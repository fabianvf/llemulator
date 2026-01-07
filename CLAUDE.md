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
   - Pattern matching on message content
   - Sequential and regex-based response selection
   - Per-token isolation for concurrent test execution

2. **Session Management** (`internal/session/`)
   - Token-scoped state isolation
   - Per-token mutex to prevent request interleaving
   - Response rules and custom models storage

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

## Quick Start Example

```bash
# Start the emulator
./openai-emulator

# Load a simple response
curl -X POST http://localhost:8080/_emulator/script \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"reset": true, "responses": "Hello from the emulator!"}'

# Use with OpenAI SDK
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Say hello"}]
  }'
```

## Script Format

The emulator uses a "send text, get text back" approach. Scripts are loaded via `POST /_emulator/script` with Bearer token auth:

### Single Response

```json
{
  "reset": true,
  "responses": "This response will be returned for any request"
}
```

### Sequential Responses

Responses are returned in order, one per request:

```json
{
  "reset": true,
  "responses": [
    "First response",
    "Second response",
    "Third response"
  ]
}
```

### Pattern-Based Responses

Use regex patterns to match specific message content:

```json
{
  "reset": true,
  "responses": {
    ".*hello.*": "Hi there!",
    ".*weather.*": "It's sunny!",
    ".*help.*": "How can I help you?"
  }
}
```

### Mixed Format

Combine sequential and pattern-based responses:

```json
{
  "reset": true,
  "responses": [
    "Default first response",
    {"pattern": ".*help.*", "response": "I can help!", "times": 2},
    "Default second response"
  ]
}
```

### Custom Models

Specify custom valid models (replaces default set):

```json
{
  "reset": true,
  "models": ["gpt-4", "claude-3", "custom-model"],
  "responses": "Test response"
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
2. Add route in `internal/server/server.go` (setupRoutes function)
3. Implement handler in `internal/server/server.go`
4. Add conformance test in both JS and Python suites

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
- Current session state
- Loaded responses and their usage counts
- Custom models if configured

## Important Implementation Notes

1. **Streaming Must Flush**: After writing each SSE event, call `Flush()` on the response writer
2. **Token Isolation**: Every token gets its own mutex - serialize requests per token
3. **Response Matching**: Pattern-based responses use regex, sequential responses are consumed in order
4. **Error Format**: Must match OpenAI error envelope for SDK exception handling
5. **Model Validation**: Invalid models return automatic 404 errors without consuming responses
6. **Default Models**: If no custom models specified, defaults include gpt-4, gpt-3.5-turbo, etc.

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
- Check if responses are exhausted (each response is used once)
- Verify regex patterns are correct (uses Go regex syntax)
- Sequential responses must be loaded in the order they'll be requested
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