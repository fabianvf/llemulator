# OpenAI API Emulator (LLEmulator)

A deterministic OpenAI API emulator for testing and CI/CD pipelines. Provides scripted responses that make official OpenAI client libraries "just work" without actual API calls.

## Features

- **Full SDK Compatibility**: Works with official OpenAI Python and JavaScript SDKs
- **Simple API**: Just send text, get text back - no complex matching rules
- **Deterministic Responses**: Script exact responses for reproducible testing
- **Streaming Support**: Full SSE streaming for chat completions and responses
- **Session Isolation**: Token-based session management for concurrent tests
- **Pattern Matching**: Optional regex patterns for dynamic responses
- **Kubernetes Ready**: Health checks, Docker support, and K8s manifests included

## Quick Start

### Run Locally

```bash
# Build and run
make run

# Or directly with Go
go run cmd/openai-emulator/main.go
```

### Run with Docker

```bash
# Build and run container
make docker-run

# Or manually
docker build -t openai-emulator .
docker run -p 8080:8080 openai-emulator
```

### Run Tests

```bash
# Run all tests (Go unit tests + conformance)
make test

# Run only conformance tests
make conformance

# Run tests against Docker container
make docker-test
```

## API Endpoints

### OpenAI Compatible (P0)
- `GET /v1/models` - List models
- `GET /v1/models/{id}` - Get model details
- `POST /v1/responses` - Text completions (streaming/non-streaming)
- `POST /v1/chat/completions` - Chat completions (streaming/non-streaming)

### Admin Control
- `POST /_emulator/script` - Load test scripts
- `POST /_emulator/reset` - Reset session state
- `GET /_emulator/state` - Debug state (requires DEBUG=true)

### Health Checks
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe

## Using with OpenAI SDKs

### Python
```python
from openai import OpenAI

client = OpenAI(
    api_key="test-token",
    base_url="http://localhost:8080/v1"
)

# Use normally
response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
```

### JavaScript
```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
    apiKey: 'test-token',
    baseURL: 'http://localhost:8080/v1'
});

// Use normally
const response = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [{ role: 'user', content: 'Hello' }]
});
```

## Scripting Responses

Load scripts via `POST /_emulator/script`:

### Simple Response
```bash
curl -X POST http://localhost:8080/_emulator/script \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{"reset": true, "responses": "Hello from the emulator!"}'
```

### Multiple Responses
```bash
curl -X POST http://localhost:8080/_emulator/script \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "reset": true,
    "responses": [
      "First response",
      "Second response",
      "Third response"
    ]
  }'
```

### Pattern-Based Responses
```bash
curl -X POST http://localhost:8080/_emulator/script \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "reset": true,
    "responses": {
      ".*hello.*": "Hi there!",
      ".*help.*": "How can I help you?",
      ".*weather.*": "It's sunny today!"
    }
  }'
```

### Tool Calls

A rule can answer with a tool call instead of text, so an agent under test
actually executes something:

```bash
curl -X POST http://localhost:8080/_emulator/script \
  -H "Authorization: Bearer test-token" \
  -H "Content-Type: application/json" \
  -d '{
    "reset": true,
    "responses": [
      {"pattern": ".*list the files.*", "tool_calls": [
        {"id": "call_1", "type": "function",
         "function": {"name": "shell", "arguments": "{\"command\":\"ls -1\"}"}}
      ]},
      {"pattern": "", "response": "all done", "times": -1}
    ]
  }'
```

The reply carries `finish_reason: "tool_calls"`, and the client is expected to
run the function and send the result back as a `role: "tool"` message.

Two things to know:

- A tool-call rule defaults to `"times": 1`. The client answers with the result
  and the last user message is unchanged, so an unlimited rule would match again
  and the conversation would never end. Put a catch-all after it, as above, to
  say what happens once the call has been made.
- `function.arguments` is a JSON document encoded as a string, which is the
  OpenAI wire format and what SDKs expect to parse.

## Kubernetes Deployment

```bash
# Apply manifests
kubectl apply -f k8s/

# Or use with your own image
kubectl set image deployment/openai-emulator openai-emulator=your-registry/openai-emulator:tag
```

## Environment Variables

- `PORT` - Server port (default: 8080)
- `DEBUG` - Enable debug endpoints (default: false)
- `EMULATOR_URL` - URL for conformance tests (default: http://localhost:8080)

## Development

See [CLAUDE.md](CLAUDE.md) for detailed development documentation.

## Testing Philosophy

This emulator prioritizes:
1. **SDK Compatibility**: Real OpenAI SDKs must work without modification
2. **Simplicity**: Send text, get text back - no complex configuration
3. **Deterministic Behavior**: Scripted responses for reproducible tests
4. **Streaming Fidelity**: Accurate SSE streaming implementation
5. **Automatic Error Handling**: Invalid models return proper OpenAI errors

## License

Apache 2.0
