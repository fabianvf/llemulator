/**
 * Integration tests for ChatService using OpenAI Emulator
 */

import { describe, it, before, after } from 'node:test';
import assert from 'node:assert';
import { ChatService } from '../src/chat-service.js';

const EMULATOR_URL = 'http://localhost:8080/v1';
const API_KEY = 'test-chat-app';

// Helper to load test scripts into emulator
async function loadTestScripts() {
  const scripts = {
    reset: true,
    rules: [
      // Helpful mode responses
      {
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: { 
            model: 'gpt-4',
            messages: [
              { role: 'system', content: 'You are a helpful AI assistant. Provide clear, accurate, and useful responses.' },
              { role: 'user', content: 'What is Node.js?' }
            ]
          }
        },
        times: 5,
        response: {
          status: 200,
          json: {
            id: 'test-helpful-1',
            object: 'chat.completion',
            model: 'gpt-4',
            choices: [{
              index: 0,
              message: {
                role: 'assistant',
                content: 'Node.js is a JavaScript runtime built on Chrome\'s V8 engine. It allows you to run JavaScript on the server-side, enabling you to build scalable network applications. Key features include non-blocking I/O, an event-driven architecture, and a rich ecosystem of packages via npm.'
              },
              finish_reason: 'stop'
            }]
          }
        }
      },
      // Creative mode responses
      {
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: {
            model: 'gpt-4',
            messages: [
              { role: 'system', content: 'You are a creative AI assistant. Be imaginative, playful, and think outside the box.' },
              { role: 'user', content: 'Tell me about dragons' }
            ]
          }
        },
        times: 5,
        response: {
          status: 200,
          json: {
            id: 'test-creative-1',
            object: 'chat.completion',
            model: 'gpt-4',
            choices: [{
              index: 0,
              message: {
                role: 'assistant',
                content: 'Ah, dragons! Those magnificent creatures that dance between myth and imagination! Picture this: scales that shimmer like liquid starlight, wings that could embrace entire clouds, and eyes that hold the wisdom of forgotten ages. In some tales, they hoard gold; in others, they hoard stories. What if dragons were actually ancient librarians, and their fire-breath was just their way of speed-reading?'
              },
              finish_reason: 'stop'
            }]
          }
        }
      },
      // Concise mode responses
      {
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: {
            model: 'gpt-4',
            messages: [
              { role: 'system', content: 'You are a concise AI assistant. Provide brief, direct answers without unnecessary elaboration.' },
              { role: 'user', content: 'What is 2+2?' }
            ]
          }
        },
        times: 5,
        response: {
          status: 200,
          json: {
            id: 'test-concise-1',
            object: 'chat.completion',
            model: 'gpt-4',
            choices: [{
              index: 0,
              message: {
                role: 'assistant',
                content: '4'
              },
              finish_reason: 'stop'
            }]
          }
        }
      },
      // Streaming response
      {
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: {
            model: 'gpt-4',
            stream: true
          }
        },
        times: 10,
        response: {
          status: 200,
          sse: [
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: { role: 'assistant' }, finish_reason: null }] } },
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: { content: 'Hello' }, finish_reason: null }] } },
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: { content: ' from' }, finish_reason: null }] } },
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: { content: ' the' }, finish_reason: null }] } },
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: { content: ' stream!' }, finish_reason: null }] } },
            { data: { id: 'stream-1', object: 'chat.completion.chunk', model: 'gpt-4', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }] } },
            { data: '[DONE]' }
          ]
        }
      },
      // Generic fallback for other requests (including summaries)
      {
        match: {
          method: 'POST',
          path: '/v1/chat/completions'
        },
        times: 100,
        response: {
          status: 200,
          json: {
            id: 'test-generic',
            object: 'chat.completion',
            model: 'gpt-4',
            choices: [{
              index: 0,
              message: {
                role: 'assistant',
                content: 'The conversation covered technical topics about Node.js and creative discussions about dragons.'
              },
              finish_reason: 'stop'
            }]
          }
        }
      }
    ]
  };
  
  const response = await fetch('http://localhost:8080/_emulator/script', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(scripts)
  });
  
  if (!response.ok) {
    throw new Error(`Failed to load test scripts: ${response.status}`);
  }
}

describe('ChatService Integration Tests', () => {
  let chatService;
  
  before(async () => {
    // Load test scripts into emulator
    await loadTestScripts();
    
    // Initialize chat service pointing to emulator
    chatService = new ChatService(API_KEY, EMULATOR_URL);
  });
  
  after(async () => {
    // Reset emulator state
    await fetch('http://localhost:8080/_emulator/reset', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${API_KEY}`
      }
    });
  });
  
  describe('Mode Management', () => {
    it('should start with helpful mode', () => {
      assert.strictEqual(chatService.mode, 'helpful');
    });
    
    it('should change modes correctly', () => {
      chatService.setMode('creative');
      assert.strictEqual(chatService.mode, 'creative');
      
      chatService.setMode('concise');
      assert.strictEqual(chatService.mode, 'concise');
      
      chatService.setMode('helpful');
      assert.strictEqual(chatService.mode, 'helpful');
    });
    
    it('should reject invalid modes', () => {
      assert.throws(
        () => chatService.setMode('invalid'),
        /Invalid mode/
      );
    });
    
    it('should return correct system prompts', () => {
      chatService.setMode('helpful');
      assert(chatService.getSystemPrompt().includes('helpful'));
      
      chatService.setMode('creative');
      assert(chatService.getSystemPrompt().includes('creative'));
      
      chatService.setMode('concise');
      assert(chatService.getSystemPrompt().includes('concise'));
    });
  });
  
  describe('Conversation Management', () => {
    it('should start with empty history', () => {
      chatService.clearHistory();
      assert.deepStrictEqual(chatService.getHistory(), []);
    });
    
    it('should maintain conversation history', async () => {
      chatService.clearHistory();
      chatService.setMode('helpful');
      
      const response = await chatService.sendMessage('What is Node.js?');
      
      const history = chatService.getHistory();
      assert.strictEqual(history.length, 2);
      assert.strictEqual(history[0].role, 'user');
      assert.strictEqual(history[0].content, 'What is Node.js?');
      assert.strictEqual(history[1].role, 'assistant');
      assert(history[1].content.includes('JavaScript runtime'));
    });
    
    it('should clear history', async () => {
      chatService.clearHistory();
      await chatService.sendMessage('Test message');
      
      assert(chatService.getHistory().length > 0);
      
      chatService.clearHistory();
      assert.strictEqual(chatService.getHistory().length, 0);
    });
  });
  
  describe('Message Sending', () => {
    it('should get helpful responses', async () => {
      chatService.clearHistory();
      chatService.setMode('helpful');
      
      const response = await chatService.sendMessage('What is Node.js?');
      
      assert(response.includes('JavaScript runtime'));
      assert(response.includes('V8 engine'));
      assert(response.includes('npm'));
    });
    
    it('should get creative responses', async () => {
      chatService.clearHistory();
      chatService.setMode('creative');
      
      const response = await chatService.sendMessage('Tell me about dragons');
      
      assert(response.includes('magnificent creatures'));
      assert(response.includes('imagination'));
      assert(response.length > 100); // Creative responses should be elaborate
    });
    
    it('should get concise responses', async () => {
      chatService.clearHistory();
      chatService.setMode('concise');
      
      const response = await chatService.sendMessage('What is 2+2?');
      
      assert.strictEqual(response, '4');
    });
    
    it('should handle errors gracefully', async () => {
      // Create a service with invalid endpoint
      const badService = new ChatService('bad-key', 'http://localhost:9999/v1');
      
      await assert.rejects(
        async () => await badService.sendMessage('Test'),
        /Failed to get AI response/
      );
      
      // History should not contain failed message
      assert.strictEqual(badService.getHistory().length, 0);
    });
  });
  
  describe('Streaming', () => {
    it('should stream responses', async () => {
      chatService.clearHistory();
      
      const chunks = [];
      const response = await chatService.streamMessage('Hello', (chunk) => {
        chunks.push(chunk);
      });
      
      assert.strictEqual(response, 'Hello from the stream!');
      assert(chunks.length > 0);
      assert.strictEqual(chunks.join(''), 'Hello from the stream!');
      
      // Check history was updated
      const history = chatService.getHistory();
      assert.strictEqual(history.length, 2);
      assert.strictEqual(history[1].content, 'Hello from the stream!');
    });
    
    it('should handle streaming errors', async () => {
      const badService = new ChatService('bad-key', 'http://localhost:9999/v1');
      
      await assert.rejects(
        async () => await badService.streamMessage('Test', () => {}),
        /Failed to stream AI response/
      );
      
      // History should not contain failed message
      assert.strictEqual(badService.getHistory().length, 0);
    });
  });
  
  describe('Summary Generation', () => {
    it('should generate summaries of conversations', async () => {
      chatService.clearHistory();
      chatService.setMode('helpful');
      
      // Have a conversation
      await chatService.sendMessage('What is Node.js?');
      
      chatService.setMode('creative');
      await chatService.sendMessage('Tell me about dragons');
      
      // Get summary
      const summary = await chatService.getSummary();
      
      assert(summary.includes('Node.js'));
      assert(summary.includes('dragons'));
    });
    
    it('should handle empty conversation', async () => {
      chatService.clearHistory();
      
      const summary = await chatService.getSummary();
      assert.strictEqual(summary, 'No conversation to summarize.');
    });
  });
});