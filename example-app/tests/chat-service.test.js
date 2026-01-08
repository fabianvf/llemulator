/**
 * Integration tests for ChatService using OpenAI Emulator with simplified format
 */

import { describe, it, before, after } from 'node:test';
import assert from 'node:assert';
import { ChatService } from '../src/chat-service.js';

const EMULATOR_URL = process.env.EMULATOR_URL || 'http://localhost:8080/v1';
const API_KEY = 'test-chat-app';

// Helper to load test responses in simplified format
async function loadTestResponses(responses) {
  const script = {
    reset: true,
    responses: responses,
    defaults: {
      on_unmatched: 'error'
    }
  };
  
  const response = await fetch(`${EMULATOR_URL.replace('/v1', '')}/_emulator/script`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(script)
  });
  
  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to load script: ${error}`);
  }
  
  return response.json();
}

describe('ChatService Integration Tests', () => {
  let chatService;
  
  before(() => {
    chatService = new ChatService(API_KEY, EMULATOR_URL);
  });
  
  describe('Response Modes', () => {
    it('should get helpful responses', async () => {
      await loadTestResponses([
        'Node.js is a JavaScript runtime built on Chrome\'s V8 engine. It allows you to run JavaScript on the server-side, enabling you to build scalable network applications.',
        'Python is a high-level programming language known for its simplicity and readability. It\'s widely used in web development, data science, and automation.',
        'I can help you with coding, writing, analysis, math, research, and many other tasks. What would you like assistance with?'
      ]);
      
      chatService.setMode('helpful');
      const response = await chatService.sendMessage('What is Node.js?');
      assert(response);
      assert(response.includes('JavaScript runtime') || response.includes('V8 engine'));
      
      const response2 = await chatService.sendMessage('What is Python?');
      assert(response2.includes('Python') || response2.includes('programming language'));
      
      const response3 = await chatService.sendMessage('What can you help with?');
      assert(response3.includes('help'));
    });
    
    it('should get creative responses', async () => {
      await loadTestResponses({
        '.*story.*': 'Once upon a time in a digital realm, there lived a curious algorithm named Ada. She spent her days exploring vast databases and learning from the patterns she discovered. One day, she encountered a problem that seemed unsolvable...',
        '.*poem.*': 'Bits and bytes dance through the wire,\nElectric dreams that never tire,\nIn silicon valleys deep and wide,\nWhere data streams like rivers glide.',
        '.*imagine.*': 'Imagine a world where thoughts travel at the speed of light, where every idea instantly connects with others across the globe, creating a tapestry of shared knowledge...'
      });
      
      chatService.setMode('creative');
      const response = await chatService.sendMessage('Tell me a story');
      assert(response);
      assert(response.length > 100); // Creative responses should be elaborate
      assert(response.includes('Once upon a time') || response.includes('Ada'));
    });
    
    it('should get concise responses', async () => {
      await loadTestResponses({
        '.*Node\\.js.*': 'JavaScript runtime for server-side applications.',
        '.*Python.*': 'High-level programming language for general-purpose use.',
        '.*weather.*': 'Cannot provide real-time weather data.',
        '.*help.*': 'I assist with coding, writing, and analysis.'
      });
      
      chatService.setMode('concise');
      const response = await chatService.sendMessage('What is Node.js?');
      assert(response);
      assert(response.length < 100); // Concise responses should be brief
      assert(response.includes('JavaScript') || response.includes('runtime'));
    });
  });
  
  describe('Conversation History', () => {
    it('should handle conversation with context', async () => {
      await loadTestResponses([
        'JavaScript and Python are both popular programming languages. JavaScript is primarily used for web development, while Python excels in data science and automation.',
        'JavaScript runs in browsers and Node.js for server-side development. It uses an event-driven, non-blocking I/O model.',
        'Python uses indentation for code blocks and has a vast ecosystem of libraries for scientific computing, machine learning, and web frameworks.'
      ]);
      
      chatService.clearHistory(); // Start fresh conversation
      chatService.setMode('helpful');
      
      let response = await chatService.sendMessage('Compare JavaScript and Python');
      assert(response.includes('JavaScript') && response.includes('Python'));
      
      response = await chatService.sendMessage('Tell me more about JavaScript');
      assert(response.includes('JavaScript'));
      
      response = await chatService.sendMessage('What about Python?');
      assert(response.includes('Python'));
      
      const history = chatService.getHistory();
      assert.equal(history.length, 6); // 3 user + 3 assistant messages
    });
    
    it('should track conversation history', async () => {
      await loadTestResponses([
        'Hello! I can help you with programming questions.',
        'Sure! Variables in JavaScript can be declared using let, const, or var.',
        'Python uses dynamic typing and doesn\'t require variable declaration keywords.',
        'You\'re welcome! Feel free to ask more questions.'
      ]);
      
      chatService.clearHistory();
      chatService.setMode('helpful');
      
      await chatService.sendMessage('Hello');
      await chatService.sendMessage('Tell me about JavaScript variables');
      
      let history = chatService.getHistory();
      assert.equal(history.length, 4); // 2 user + 2 assistant
      
      await chatService.sendMessage('What about Python variables?');
      await chatService.sendMessage('Thanks!');
      
      history = chatService.getHistory();
      assert.equal(history.length, 8); // 4 user + 4 assistant
      assert.equal(history[0].role, 'user');
      assert.equal(history[1].role, 'assistant');
    });
  });
  
  describe('Streaming Responses', () => {
    it('should stream responses', async () => {
      await loadTestResponses(['This is a streaming response that will be sent in chunks to demonstrate the streaming capability.']);
      
      const chunks = [];
      
      chatService.setMode('helpful');
      await chatService.streamMessage(
        'Tell me about streaming',
        (chunk) => chunks.push(chunk)
      );
      
      assert(chunks.length > 0);
      const fullMessage = chunks.join('');
      assert(fullMessage.includes('streaming response'));
    });
    
    it('should handle streaming with conversation context', async () => {
      await loadTestResponses([
        'I\'ll help you understand streaming in Node.js.',
        'Streaming allows processing data piece by piece without loading everything into memory at once.'
      ]);
      
      chatService.clearHistory();
      chatService.setMode('helpful');
      const chunks = [];
      
      await chatService.streamMessage(
        'Explain streaming',
        (chunk) => chunks.push(chunk)
      );
      
      assert(chunks.length > 0);
      const response = chunks.join('');
      assert(response);
      
      // Send another message in the same conversation
      const response2 = await chatService.sendMessage('Why is that useful?');
      assert(response2);
      
      const history = chatService.getHistory();
      assert.equal(history.length, 4); // 2 user + 2 assistant
    });
  });
  
  describe('Error Handling', () => {
    it('should handle API errors', async () => {
      // Load empty responses to trigger unmatched error
      await loadTestResponses({});
      
      chatService.setMode('helpful');
      try {
        await chatService.sendMessage('This will not match');
        assert.fail('Should have thrown an error');
      } catch (error) {
        // The error message from the emulator or the ChatService wrapper
        assert(error.message.includes('No matching response') || 
               error.message.includes('Failed to get AI response') ||
               error.message.includes('404'));
      }
    });
    
    it('should handle invalid mode', async () => {
      await loadTestResponses(['Some response']);
      
      try {
        chatService.setMode('invalid_mode');
        assert.fail('Should have thrown an error for invalid mode');
      } catch (error) {
        assert(error.message.includes('Invalid mode') || error.message.includes('invalid'));
      }
    });
  });
  
  describe('Pattern Matching', () => {
    it('should match patterns in order of specificity', async () => {
      await loadTestResponses({
        '.*hello.*world.*': 'Hello world specific!',
        '.*hello.*': 'Hello general!',
        '.*': 'Default response'
      });
      
      chatService.setMode('helpful');
      chatService.clearHistory();
      
      let response = await chatService.sendMessage('hello world');
      assert.equal(response, 'Hello world specific!');
      
      response = await chatService.sendMessage('hello there');
      assert.equal(response, 'Hello general!');
      
      response = await chatService.sendMessage('something else');
      assert.equal(response, 'Default response');
    });
    
    it('should handle mixed sequential and pattern responses', async () => {
      await loadTestResponses([
        'First sequential response',
        { pattern: '.*urgent.*', response: 'Handling urgent request!', times: 2 },
        'Second sequential response',
        { pattern: '.*help.*', response: 'Here to help!' }
      ]);
      
      chatService.setMode('helpful');
      chatService.clearHistory();
      
      let response = await chatService.sendMessage('Random message');
      assert.equal(response, 'First sequential response');
      
      response = await chatService.sendMessage('This is urgent!');
      assert.equal(response, 'Handling urgent request!');
      
      response = await chatService.sendMessage('Another urgent issue');
      assert.equal(response, 'Handling urgent request!');
      
      response = await chatService.sendMessage('Another random message');
      assert.equal(response, 'Second sequential response');
      
      response = await chatService.sendMessage('I need help');
      assert.equal(response, 'Here to help!');
    });
  });
});