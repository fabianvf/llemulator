import { describe, it, beforeEach } from 'node:test';
import assert from 'node:assert';
import OpenAI from 'openai';

const EMULATOR_URL = process.env.EMULATOR_URL || 'http://localhost:8080';
const API_KEY = 'test-token-js';

// Helper to load responses in the simple format
async function withResponses(responses, testFn) {
  await loadScript({ responses });
  return testFn();
}

// Helper to load advanced rules
async function withRules(rules, testFn) {
  await loadScript({ rules });
  return testFn();
}

async function loadScript(config) {
  const script = {
    reset: true,
    ...config,
    defaults: {
      on_unmatched: 'error',
      ...(config.defaults || {}),
    },
  };
  
  const response = await fetch(`${EMULATOR_URL}/_emulator/script`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(script),
  });
  
  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to load test script: ${response.status} - ${error}`);
  }
}

describe('OpenAI Emulator - Simple Format Tests', () => {
  let openai;
  
  beforeEach(() => {
    openai = new OpenAI({
      apiKey: API_KEY,
      baseURL: `${EMULATOR_URL}/v1`,
    });
  });
  
  describe('Basic Response Tests', () => {
    it('should handle a single response', async () => {
      await withResponses(['Hello! How can I help you today?'], async () => {
        const completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Say hello' }],
        });
        
        assert.equal(
          completion.choices[0].message.content,
          'Hello! How can I help you today?'
        );
      });
    });
    
    it('should handle sequential responses', async () => {
      await withResponses(['First', 'Second', 'Third'], async () => {
        // First request
        let completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test 1' }],
        });
        assert.equal(completion.choices[0].message.content, 'First');
        
        // Second request
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test 2' }],
        });
        assert.equal(completion.choices[0].message.content, 'Second');
        
        // Third request
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test 3' }],
        });
        assert.equal(completion.choices[0].message.content, 'Third');
      });
    });
    
    it('should handle pattern-based responses', async () => {
      const patterns = {
        '.*hello.*': 'Hi there!',
        '.*weather.*': 'It\'s sunny today!',
        '\\d+\\s*\\+\\s*\\d+': 'I can\'t do math!',
        'bye': 'Goodbye!'
      };
      
      await withResponses(patterns, async () => {
        // Test hello pattern
        let completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Say hello please' }],
        });
        assert.equal(completion.choices[0].message.content, 'Hi there!');
        
        // Test weather pattern
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'What\'s the weather?' }],
        });
        assert.equal(completion.choices[0].message.content, 'It\'s sunny today!');
        
        // Test math pattern
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'What is 2 + 2?' }],
        });
        assert.equal(completion.choices[0].message.content, 'I can\'t do math!');
        
        // Test exact match
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'bye' }],
        });
        assert.equal(completion.choices[0].message.content, 'Goodbye!');
      });
    });
  });
  
  describe('Streaming Tests', () => {
    it('should automatically handle streaming with same response', async () => {
      await withResponses(['One, two, three'], async () => {
        const stream = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Count to 3' }],
          stream: true,
        });
        
        let content = '';
        for await (const chunk of stream) {
          if (chunk.choices[0]?.delta?.content) {
            content += chunk.choices[0].delta.content;
          }
        }
        
        assert(content.includes('One'));
        assert(content.includes('two'));
        assert(content.includes('three'));
      });
    });
    
    it('should work for both streaming and non-streaming', async () => {
      await withResponses(['Same content for both'], async () => {
        // Non-streaming
        const completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test' }],
        });
        assert.equal(completion.choices[0].message.content, 'Same content for both');
        
        // Streaming with same script
        const stream = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test' }],
          stream: true,
        });
        
        let streamContent = '';
        for await (const chunk of stream) {
          if (chunk.choices[0]?.delta?.content) {
            streamContent += chunk.choices[0].delta.content;
          }
        }
        
        assert(streamContent.includes('Same content'));
      });
    });
  });
  
  describe('Conversation Flow', () => {
    it('should handle a typical conversation', async () => {
      const conversation = [
        'Welcome! I\'m your AI assistant.',
        'I can help you with various tasks.',
        'What would you like to know?',
        'That\'s an interesting question!',
        'Is there anything else?'
      ];
      
      await withResponses(conversation, async () => {
        const messages = [
          'Hello',
          'What can you do?',
          'Tell me something',
          'Why is the sky blue?',
          'Thanks'
        ];
        
        for (let i = 0; i < messages.length; i++) {
          const completion = await openai.chat.completions.create({
            model: 'gpt-4',
            messages: [{ role: 'user', content: messages[i] }],
          });
          assert.equal(completion.choices[0].message.content, conversation[i]);
        }
      });
    });
  });
  
  describe('Mixed Format', () => {
    it('should handle mixed sequential and pattern responses', async () => {
      await withResponses([
        'Default first response',
        { match: 'help', response: 'How can I help?' },
        { match: 'error', error: 'Something went wrong', status: 500 },
      ], async () => {
        // First request gets default
        let completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Random' }],
        });
        assert.equal(completion.choices[0].message.content, 'Default first response');
        
        // Pattern match
        completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'I need help' }],
        });
        assert.equal(completion.choices[0].message.content, 'How can I help?');
      });
    });
  });
  
  describe('API Conformance', () => {
    it('should generate proper response structure', async () => {
      await withResponses(['Test response'], async () => {
        const completion = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test' }],
        });
        
        // Verify structure
        assert(completion.id);
        assert.equal(completion.object, 'chat.completion');
        assert.equal(completion.model, 'gpt-4');
        assert(completion.created);
        assert(completion.choices);
        assert.equal(completion.choices.length, 1);
        
        const choice = completion.choices[0];
        assert.equal(choice.index, 0);
        assert.equal(choice.message.role, 'assistant');
        assert.equal(choice.message.content, 'Test response');
        assert.equal(choice.finish_reason, 'stop');
        
        // Usage should be generated
        assert(completion.usage);
        assert(completion.usage.total_tokens > 0);
      });
    });
    
    it('should generate proper streaming structure', async () => {
      await withResponses(['Streaming test'], async () => {
        const stream = await openai.chat.completions.create({
          model: 'gpt-4',
          messages: [{ role: 'user', content: 'Test' }],
          stream: true,
        });
        
        const chunks = [];
        for await (const chunk of stream) {
          chunks.push(chunk);
        }
        
        assert(chunks.length > 0);
        
        // First chunk should have role
        assert.equal(chunks[0].object, 'chat.completion.chunk');
        assert.equal(chunks[0].choices[0].delta.role, 'assistant');
        
        // Last chunk should have finish reason
        const lastChunk = chunks[chunks.length - 1];
        assert.equal(lastChunk.choices[0].finish_reason, 'stop');
      });
    });
  });
});

describe('OpenAI Emulator - Text Completion', () => {
  describe('Responses API', () => {
    it('should handle text completion', async () => {
      await withRules([{
        match: { method: 'POST', path: '/v1/responses' },
        times: 10,
        response: {
          status: 200,
          content: 'This is a test response.',
        },
      }], async () => {
        const response = await fetch(`${EMULATOR_URL}/v1/responses`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${API_KEY}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            model: 'gpt-4',
            prompt: 'Hello, world!',
            max_tokens: 100,
          }),
        });
        
        assert(response.ok);
        const data = await response.json();
        assert.equal(data.choices[0].text, 'This is a test response.');
      });
    });
    
    it('should handle text completion streaming', async () => {
      await withRules([{
        match: { method: 'POST', path: '/v1/responses' },
        times: 10,
        response: {
          status: 200,
          content: 'Streaming works!',
        },
      }], async () => {
        const response = await fetch(`${EMULATOR_URL}/v1/responses`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${API_KEY}`,
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            model: 'gpt-4',
            prompt: 'Test',
            max_tokens: 100,
            stream: true,
          }),
        });
        
        assert(response.ok);
        assert(response.headers.get('content-type').includes('text/event-stream'));
        
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let content = '';
        
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          
          const text = decoder.decode(value, { stream: true });
          const lines = text.split('\n');
          
          for (const line of lines) {
            if (line.startsWith('data: ') && line !== 'data: [DONE]') {
              try {
                const event = JSON.parse(line.slice(6));
                if (event.choices?.[0]?.text) {
                  content += event.choices[0].text;
                }
              } catch (e) {}
            }
          }
        }
        
        assert(content.includes('Streaming'));
        assert(content.includes('works'));
      });
    });
  });
});

describe('OpenAI Emulator - Models & Errors', () => {
  let openai;
  
  beforeEach(() => {
    openai = new OpenAI({
      apiKey: API_KEY,
      baseURL: `${EMULATOR_URL}/v1`,
    });
  });
  
  describe('Models API', () => {
    it('should list models', async () => {
      await withRules([{
        match: { method: 'GET', path: '/v1/models' },
        times: 1,
        response: {
          status: 200,
          json: {
            object: 'list',
            data: [
              { id: 'gpt-4', object: 'model', created: 1687882410, owned_by: 'openai' },
              { id: 'gpt-3.5-turbo', object: 'model', created: 1687882410, owned_by: 'openai' },
            ],
          },
        },
      }], async () => {
        const models = await openai.models.list();
        assert.equal(models.data.length, 2);
        assert.equal(models.data[0].id, 'gpt-4');
        assert.equal(models.data[1].id, 'gpt-3.5-turbo');
      });
    });
    
    it('should retrieve a model', async () => {
      await withRules([{
        match: { method: 'GET', path: '/v1/models/gpt-4' },
        times: 1,
        response: {
          status: 200,
          content: 'gpt-4',
        },
      }], async () => {
        const model = await openai.models.retrieve('gpt-4');
        assert.equal(model.id, 'gpt-4');
        assert.equal(model.object, 'model');
      });
    });
  });
  
  describe('Error Handling', () => {
    it('should handle invalid model error', async () => {
      await withRules([{
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: { model: 'invalid-model' }
        },
        times: 1,
        response: {
          status: 404,
          json: {
            error: {
              message: 'The model `invalid-model` does not exist',
              type: 'invalid_request_error',
              param: 'model',
              code: 'model_not_found',
            },
          },
        },
      }], async () => {
        try {
          await openai.chat.completions.create({
            model: 'invalid-model',
            messages: [{ role: 'user', content: 'Test' }],
          });
          assert.fail('Should have thrown an error');
        } catch (error) {
          assert(error);
          assert(error.status === 404 || error.status === 400);
        }
      });
    });
    
    it('should handle missing authorization', async () => {
      const unauthorizedClient = new OpenAI({
        apiKey: '',
        baseURL: `${EMULATOR_URL}/v1`,
      });
      
      try {
        await unauthorizedClient.models.list();
        assert.fail('Should have thrown an error');
      } catch (error) {
        assert(error);
      }
    });
  });
});