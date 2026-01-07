# AI Chat Application - Example for OpenAI Emulator Testing

This is a demonstration application showing how to build and test LLM-powered applications using the OpenAI Emulator for deterministic testing.

## 🎯 Purpose

This example demonstrates:
- Building a real-world LLM application with OpenAI SDK
- Writing comprehensive integration tests
- Using the emulator for CI/CD testing without API costs
- Testing different AI behaviors (modes, streaming, error handling)

## 🏗️ Architecture

```
example-app/
├── src/
│   ├── chat-service.js    # Core business logic
│   └── cli.js             # Interactive CLI interface
├── tests/
│   └── chat-service.test.js  # Integration tests
├── scripts/
│   └── run-tests.sh       # Test runner script
├── docker-compose.yml     # Docker test environment
└── package.json
```

## 🚀 Quick Start

### Prerequisites
- Node.js 20+
- OpenAI Emulator built and available in parent directory

### Installation
```bash
npm install
```

### Running the CLI Application

1. **With Real OpenAI API:**
```bash
export OPENAI_API_KEY=your-api-key
npm start
```

2. **With Emulator (for development/testing):**
```bash
# Terminal 1: Start emulator
cd ..
./openai-emulator

# Terminal 2: Run app
export OPENAI_BASE_URL=http://localhost:8080/v1
export OPENAI_API_KEY=test-key
npm start
```

### CLI Commands
- `/mode [helpful|creative|concise]` - Change AI personality
- `/history` - View conversation history
- `/clear` - Clear conversation history
- `/summary` - Generate conversation summary
- `/exit` - Exit application

## 🧪 Testing

### Run Tests with Local Emulator

```bash
# Start emulator first
cd ..
./openai-emulator &

# Run tests
cd example-app
npm test
```

### Run Tests with Script
```bash
./scripts/run-tests.sh
```

### Run Tests with Docker Compose
```bash
docker-compose up --build --abort-on-container-exit
```

## 📋 Test Coverage

The test suite covers:

### ✅ Mode Management
- Starting with default mode
- Switching between modes (helpful, creative, concise)
- Validating mode-specific behaviors
- Error handling for invalid modes

### ✅ Conversation Management
- Maintaining conversation history
- Clearing history
- History isolation between sessions

### ✅ Message Handling
- Sending and receiving messages
- Mode-specific responses
- Error recovery
- History consistency

### ✅ Streaming
- Real-time streaming responses
- Chunk accumulation
- History updates after streaming

### ✅ Summary Generation
- Summarizing conversations
- Handling empty conversations

## 🐳 Docker Testing

The Docker Compose setup provides:
- Isolated test environment
- Automatic emulator startup
- Health checks
- Network isolation
- CI/CD ready configuration

```yaml
services:
  emulator:      # OpenAI API emulator
  app-tests:     # Test runner container
```

## 🔧 CI/CD Integration

Example GitHub Actions workflow included:
```yaml
- Integration tests with emulator
- Docker Compose tests
- CLI smoke tests
```

## 📊 Test Patterns

### 1. Deterministic Responses
```javascript
// Load scripted responses into emulator
const scripts = {
  rules: [{
    match: { 
      method: 'POST', 
      path: '/v1/chat/completions',
      json: { model: 'gpt-4', /* ... */ }
    },
    response: { /* deterministic response */ }
  }]
};
```

### 2. Mode Testing
```javascript
// Test different AI personalities
chatService.setMode('creative');
const response = await chatService.sendMessage('Tell me a story');
assert(response.includes('imagination'));
```

### 3. Streaming Testing
```javascript
// Test streaming with callbacks
const chunks = [];
await chatService.streamMessage('Hello', chunk => {
  chunks.push(chunk);
});
assert(chunks.length > 0);
```

### 4. Error Simulation
```javascript
// Test error handling
const badService = new ChatService('bad-key', 'invalid-url');
await assert.rejects(
  () => badService.sendMessage('Test'),
  /Failed to get AI response/
);
```

## 🎓 Learning Points

This example teaches:

1. **Test-Driven LLM Development**
   - Write tests first
   - Use emulator for fast feedback
   - Ensure deterministic behavior

2. **Cost-Effective Testing**
   - No API costs during development
   - Unlimited test runs in CI/CD
   - Fast local testing

3. **Comprehensive Coverage**
   - Test all code paths
   - Simulate edge cases
   - Verify error handling

4. **Production Patterns**
   - Mode switching for different use cases
   - Conversation history management
   - Streaming for better UX
   - Graceful error recovery

## 🚦 Best Practices

1. **Always test with emulator first**
   - Faster feedback loop
   - No API costs
   - Deterministic results

2. **Use scripted responses for specific scenarios**
   - Test edge cases
   - Verify error handling
   - Ensure consistent behavior

3. **Test streaming and non-streaming**
   - Both have different failure modes
   - Streaming improves UX
   - Non-streaming is simpler

4. **Maintain conversation context**
   - History affects responses
   - Test with various contexts
   - Clear history between test cases

## 📚 Resources

- [OpenAI SDK Documentation](https://github.com/openai/openai-node)
- [OpenAI Emulator Documentation](../README.md)
- [Node.js Testing Best Practices](https://nodejs.org/api/test.html)

## 📝 License

MIT - Use freely for learning and testing!