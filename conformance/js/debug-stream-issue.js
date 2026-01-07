import OpenAI from 'openai';

console.log('=== Debugging Streaming Issue ===\n');

// Start server first if needed
console.log('Make sure server is running on port 8080 with DEBUG=true\n');

const token = 'debug-stream-test';

// Load a simple streaming script
async function loadScript() {
  const response = await fetch('http://localhost:8080/_emulator/script', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      reset: true,
      rules: [
        {
          match: {
            method: 'POST',
            path: '/v1/chat/completions',
            json: { stream: true }
          },
          times: 10,
          response: {
            status: 200,
            sse: [
              { data: { id: 'test1', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{ index: 0, delta: { role: 'assistant' }, finish_reason: null }] } },
              { data: { id: 'test1', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{ index: 0, delta: { content: 'Hello' }, finish_reason: null }] } },
              { data: { id: 'test1', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }] } },
              { data: '[DONE]' }
            ]
          }
        }
      ]
    })
  });
  
  console.log('Script loaded:', response.status === 200 ? 'SUCCESS' : 'FAILED');
}

// Test with raw fetch to see SSE
async function testRawSSE() {
  console.log('\n1. Testing with raw fetch:');
  
  const response = await fetch('http://localhost:8080/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'test' }],
      stream: true
    })
  });
  
  console.log('Response status:', response.status);
  console.log('Content-Type:', response.headers.get('content-type'));
  
  if (response.ok) {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    
    console.log('\nRaw SSE data:');
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      
      const chunk = decoder.decode(value, { stream: true });
      buffer += chunk;
      console.log('CHUNK:', JSON.stringify(chunk));
    }
    
    console.log('\nFull buffer:', buffer);
    
    // Parse events
    const lines = buffer.split('\n');
    const events = [];
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const data = line.slice(6);
        console.log('EVENT DATA:', data);
        if (data !== '[DONE]') {
          try {
            events.push(JSON.parse(data));
          } catch (e) {
            console.log('Failed to parse:', data);
          }
        }
      }
    }
    
    console.log('\nParsed events:', events.length);
  }
}

// Test with OpenAI SDK
async function testSDK() {
  console.log('\n2. Testing with OpenAI SDK:');
  
  const openai = new OpenAI({
    apiKey: token,
    baseURL: 'http://localhost:8080/v1'
  });
  
  try {
    console.log('Creating stream...');
    const stream = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'test' }],
      stream: true
    });
    
    console.log('Stream created, type:', typeof stream);
    console.log('Stream constructor:', stream.constructor.name);
    
    const chunks = [];
    console.log('\nIterating chunks:');
    for await (const chunk of stream) {
      console.log('CHUNK:', JSON.stringify(chunk));
      chunks.push(chunk);
    }
    
    console.log('\nTotal chunks received:', chunks.length);
    
    if (chunks.length === 0) {
      console.log('ERROR: No chunks received!');
      console.log('Stream object:', stream);
    }
  } catch (error) {
    console.error('SDK Error:', error);
    console.error('Error type:', error.constructor.name);
    if (error.response) {
      console.error('Response status:', error.response.status);
      console.error('Response headers:', error.response.headers);
    }
  }
}

// Run tests
await loadScript();
await testRawSSE();
await testSDK();

console.log('\n=== Debug Complete ===');