import OpenAI from 'openai';

// Test against real OpenAI to see the format
async function debugRealOpenAI() {
  // This would test against real OpenAI API to see format
  // But we'll simulate what we know the format should be
  
  console.log('Expected SSE format from OpenAI:');
  console.log('data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}');
  console.log('data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}');
  console.log('data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}');
  console.log('data: [DONE]');
}

// Test our emulator
async function testEmulator() {
  const openai = new OpenAI({
    apiKey: 'test-debug',
    baseURL: 'http://localhost:8080/v1'
  });
  
  // First load a test script
  const scriptResponse = await fetch('http://localhost:8080/_emulator/script', {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer test-debug',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      reset: true,
      rules: [{
        match: {
          method: 'POST',
          path: '/v1/chat/completions',
          json: {stream: true}
        },
        times: 10,
        response: {
          status: 200,
          sse: [
            {data: {id: 'chatcmpl-123', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{index: 0, delta: {role: 'assistant'}, finish_reason: null}]}},
            {data: {id: 'chatcmpl-123', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{index: 0, delta: {content: 'Hello'}, finish_reason: null}]}},
            {data: {id: 'chatcmpl-123', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{index: 0, delta: {content: ' world'}, finish_reason: null}]}},
            {data: {id: 'chatcmpl-123', object: 'chat.completion.chunk', created: 1234567890, model: 'gpt-4', choices: [{index: 0, delta: {}, finish_reason: 'stop'}]}},
            {data: '[DONE]'}
          ]
        }
      }]
    })
  });
  
  console.log('Script loaded:', await scriptResponse.text());
  
  try {
    console.log('\nTesting streaming with OpenAI SDK:');
    const stream = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [{role: 'user', content: 'test'}],
      stream: true
    });
    
    const chunks = [];
    for await (const chunk of stream) {
      chunks.push(chunk);
      console.log('Chunk received:', JSON.stringify(chunk));
    }
    
    console.log(`\nTotal chunks received: ${chunks.length}`);
    
    // Accumulate content
    let fullContent = '';
    for (const chunk of chunks) {
      if (chunk.choices[0]?.delta?.content) {
        fullContent += chunk.choices[0].delta.content;
      }
    }
    console.log('Full content:', fullContent);
    
  } catch (error) {
    console.error('Error:', error.message);
    if (error.response) {
      console.error('Response body:', await error.response.text());
    }
  }
}

// Run the debug
await debugRealOpenAI();
console.log('\n---\n');
await testEmulator();