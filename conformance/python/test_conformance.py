"""Python conformance tests for OpenAI emulator."""

import os
import json
import pytest
import httpx
from openai import OpenAI
from functools import wraps

EMULATOR_URL = os.getenv('EMULATOR_URL', 'http://localhost:8080')
API_KEY = 'test-token-python'


def load_responses(responses):
    """Decorator to load responses before a test."""
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            # Load the responses
            script = {
                "reset": True,
                "responses": responses,
                "defaults": {"on_unmatched": "error"},
            }
            
            response = httpx.post(
                f"{EMULATOR_URL}/_emulator/script",
                headers={"Authorization": f"Bearer {API_KEY}"},
                json=script,
            )
            
            if response.status_code != 200:
                raise Exception(f"Failed to load test script: {response.status_code} - {response.text}")
            
            # Run the test
            return func(*args, **kwargs)
        return wrapper
    return decorator


def load_rules(rules):
    """Decorator for advanced rule-based responses (models, errors, etc)."""
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            script = {
                "reset": True,
                "rules": rules,
                "defaults": {"on_unmatched": "error"},
            }
            
            response = httpx.post(
                f"{EMULATOR_URL}/_emulator/script",
                headers={"Authorization": f"Bearer {API_KEY}"},
                json=script,
            )
            
            if response.status_code != 200:
                raise Exception(f"Failed to load test script: {response.status_code} - {response.text}")
            
            return func(*args, **kwargs)
        return wrapper
    return decorator


@pytest.fixture
def client():
    """Create OpenAI client configured for emulator."""
    return OpenAI(
        api_key=API_KEY,
        base_url=f"{EMULATOR_URL}/v1",
    )


class TestSimpleFormat:
    """Test the simple, recommended format."""
    
    @load_responses(["Hello! How can I help you today?"])
    def test_single_response(self, client):
        """Simple single response test."""
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Say hello"}],
        )
        assert completion.choices[0].message.content == "Hello! How can I help you today?"
    
    @load_responses(["First response", "Second response", "Third response"])
    def test_sequential_responses(self, client):
        """Sequential responses returned in order."""
        # First request
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Test 1"}],
        )
        assert completion.choices[0].message.content == "First response"
        
        # Second request
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Test 2"}],
        )
        assert completion.choices[0].message.content == "Second response"
        
        # Third request
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Test 3"}],
        )
        assert completion.choices[0].message.content == "Third response"
    
    @load_responses({
        ".*hello.*": "Hi there!",
        ".*weather.*": "It's sunny today!",
        "\\d+\\s*\\+\\s*\\d+": "I can't do math!",
        "bye": "Goodbye!"
    })
    def test_pattern_matching(self, client):
        """Pattern-based responses using regex."""
        # Test hello pattern
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Say hello please"}],
        )
        assert completion.choices[0].message.content == "Hi there!"
        
        # Test weather pattern
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "What's the weather like?"}],
        )
        assert completion.choices[0].message.content == "It's sunny today!"
        
        # Test math pattern
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "What is 2 + 2?"}],
        )
        assert completion.choices[0].message.content == "I can't do math!"
        
        # Test exact match
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "bye"}],
        )
        assert completion.choices[0].message.content == "Goodbye!"
    
    @load_responses(["One, two, three"])
    def test_streaming_automatic(self, client):
        """Same response works for streaming automatically."""
        stream = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Count to 3"}],
            stream=True,
        )
        
        content = ''
        for chunk in stream:
            if chunk.choices[0].delta and hasattr(chunk.choices[0].delta, 'content'):
                if chunk.choices[0].delta.content:
                    content += chunk.choices[0].delta.content
        
        # Content should match what we provided
        assert "One" in content and "two" in content and "three" in content
    
    @load_responses([
        "Welcome! I'm your AI assistant.",
        "I can help you with various tasks.",
        "What would you like to know?",
        "That's an interesting question!",
        "Is there anything else?"
    ])
    def test_conversation_flow(self, client):
        """Test a typical conversation flow."""
        messages = [
            "Hello",
            "What can you do?",
            "Tell me something",
            "Why is the sky blue?",
            "Thanks"
        ]
        
        expected = [
            "Welcome! I'm your AI assistant.",
            "I can help you with various tasks.",
            "What would you like to know?",
            "That's an interesting question!",
            "Is there anything else?"
        ]
        
        for msg, exp in zip(messages, expected):
            completion = client.chat.completions.create(
                model='gpt-4',
                messages=[{"role": "user", "content": msg}],
            )
            assert completion.choices[0].message.content == exp


class TestMixedFormat:
    """Test mixed sequential and pattern-based responses."""
    
    @load_responses([
        "Default first response",
        {"match": "help", "response": "How can I help?"},
        {"match": "error", "error": "Something went wrong", "status": 500},
    ])
    def test_mixed_responses(self, client):
        """Mix of sequential and pattern-based responses."""
        # First request gets default
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Random message"}],
        )
        assert completion.choices[0].message.content == "Default first response"
        
        # Request matching pattern
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "I need help"}],
        )
        assert completion.choices[0].message.content == "How can I help?"


class TestCompletionEndpoint:
    """Test text completion endpoint with simple format."""
    
    @load_rules([{
        "match": {"method": "POST", "path": "/v1/responses"},
        "times": 10,
        "response": {
            "status": 200,
            "content": "This is a test response.",
        },
    }])
    def test_text_completion(self):
        """Text completion with auto-wrapping."""
        response = httpx.post(
            f"{EMULATOR_URL}/v1/responses",
            headers={
                "Authorization": f"Bearer {API_KEY}",
                "Content-Type": "application/json",
            },
            json={
                "model": "gpt-4",
                "prompt": "Hello, world!",
                "max_tokens": 100,
            },
        )
        
        assert response.status_code == 200
        data = response.json()
        assert data['choices'][0]['text'] == "This is a test response."
    
    @load_rules([{
        "match": {"method": "POST", "path": "/v1/responses"},
        "times": 10,
        "response": {
            "status": 200,
            "content": "Streaming works too!",
        },
    }])
    def test_text_completion_streaming(self):
        """Text completion streaming with auto-wrapping."""
        with httpx.stream(
            'POST',
            f"{EMULATOR_URL}/v1/responses",
            headers={
                "Authorization": f"Bearer {API_KEY}",
                "Content-Type": "application/json",
            },
            json={
                "model": "gpt-4",
                "prompt": "Test",
                "max_tokens": 100,
                "stream": True,
            },
        ) as response:
            assert response.status_code == 200
            assert 'text/event-stream' in response.headers.get('content-type', '')
            
            content = ''
            for line in response.iter_lines():
                if line.startswith('data: '):
                    data = line[6:]
                    if data != '[DONE]':
                        try:
                            event = json.loads(data)
                            if event.get('choices') and event['choices'][0].get('text'):
                                content += event['choices'][0]['text']
                        except json.JSONDecodeError:
                            pass
            
            assert "Streaming" in content and "works" in content


class TestAPIConformance:
    """Test that OpenAI SDK features work correctly."""
    
    @load_responses(["Test response with all fields"])
    def test_response_structure(self, client):
        """Verify auto-generated response has correct structure."""
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Test"}],
        )
        
        # Check all expected fields exist
        assert completion.id is not None
        assert completion.object == 'chat.completion'
        assert completion.model == 'gpt-4'
        assert completion.created is not None
        assert completion.choices is not None
        assert len(completion.choices) == 1
        
        choice = completion.choices[0]
        assert choice.index == 0
        assert choice.message.role == 'assistant'
        assert choice.message.content == "Test response with all fields"
        assert choice.finish_reason == 'stop'
        
        # Usage info should be generated
        assert completion.usage is not None
        assert completion.usage.total_tokens > 0
    
    @load_responses(["Streaming test"])
    def test_streaming_structure(self, client):
        """Verify streaming response structure."""
        stream = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "Test"}],
            stream=True,
        )
        
        chunks = list(stream)
        assert len(chunks) > 0
        
        # First chunk should have role
        first_chunk = chunks[0]
        assert first_chunk.object == 'chat.completion.chunk'
        assert first_chunk.choices[0].delta.role == 'assistant'
        
        # Last chunk should have finish_reason
        last_chunk = chunks[-1]
        assert last_chunk.choices[0].finish_reason == 'stop'


class TestModelsAPI:
    """Models API still needs traditional format."""
    
    @load_rules([{
        "match": {"method": "GET", "path": "/v1/models"},
        "times": 1,
        "response": {
            "status": 200,
            "json": {
                "object": "list",
                "data": [
                    {"id": "gpt-4", "object": "model", "created": 1687882410, "owned_by": "openai"},
                    {"id": "gpt-3.5-turbo", "object": "model", "created": 1687882410, "owned_by": "openai"},
                ],
            },
        },
    }])
    def test_list_models(self, client):
        """List available models."""
        models = client.models.list()
        assert len(models.data) == 2
        assert models.data[0].id == 'gpt-4'
        assert models.data[1].id == 'gpt-3.5-turbo'
    
    @load_rules([{
        "match": {"method": "GET", "path": "/v1/models/gpt-4"},
        "times": 1,
        "response": {
            "status": 200,
            "content": "gpt-4",
        },
    }])
    def test_retrieve_model(self, client):
        """Retrieve specific model."""
        model = client.models.retrieve('gpt-4')
        assert model.id == 'gpt-4'
        assert model.object == 'model'


class TestErrorHandling:
    """Error handling tests."""
    
    @load_rules([{
        "match": {
            "method": "POST",
            "path": "/v1/chat/completions",
            "json": {"model": "invalid-model"}
        },
        "times": 1,
        "response": {
            "status": 404,
            "json": {
                "error": {
                    "message": "The model `invalid-model` does not exist",
                    "type": "invalid_request_error",
                    "param": "model",
                    "code": "model_not_found",
                },
            },
        },
    }])
    def test_invalid_model(self, client):
        """Test error for invalid model."""
        with pytest.raises(Exception) as exc_info:
            client.chat.completions.create(
                model='invalid-model',
                messages=[{"role": "user", "content": "Test"}],
            )
        assert exc_info.value is not None
    
    def test_missing_authorization(self):
        """Test missing auth token."""
        unauthorized_client = OpenAI(
            api_key='',
            base_url=f"{EMULATOR_URL}/v1",
        )
        
        with pytest.raises(Exception):
            unauthorized_client.models.list()