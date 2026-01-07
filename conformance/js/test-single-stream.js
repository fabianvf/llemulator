import OpenAI from 'openai';

const API_KEY = 'test-token-js';
const EMULATOR_URL = 'http://localhost:8080';

// Load the same script as the test
async function loadTestScripts() {
  const script = {
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
            { data: { id: 'chatcmpl-test456', object: 'chat.completion.chunk', created: 1701234567, model: 'gpt-4', choices: [{ index: 0, delta: { role: 'assistant' }, finish_reason: null }] } },
            { data: { id: 'chatcmpl-test456', object: 'chat.completion.chunk', created: 1701234567, model: 'gpt-4', choices: [{ index: 0, delta: { content: 'One' }, finish_reason: null }] } },
            { data: { id: 'chatcmpl-test456', object: 'chat.completion.chunk', created: 1701234567, model: 'gpt-4', choices: [{ index: 0, delta: { content: ', two' }, finish_reason: null }] } },
            { data: { id: 'chatcmpl-test456', object: 'chat.completion.chunk', created: 1701234567, model: 'gpt-4', choices: [{ index: 0, delta: { content: ', three' }, finish_reason: null }] } },
            { data: { id: 'chatcmpl-test456', object: 'chat.completion.chunk', created: 1701234567, model: 'gpt-4', choices: [{ index: 0, delta: {}, finish_reason: 'stop' }] } },
            { data: '[DONE]' },
          ],
        },
      },
    ],
  };
  
  const response = await fetch(`${EMULATOR_URL}/_emulator/script`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(script),
  });
  
  console.log('Script loaded:', response.status === 200 ? 'SUCCESS' : 'FAILED');
}

// Run the exact test
async function runTest() {
  const openai = new OpenAI({
    apiKey: API_KEY,
    baseURL: `${EMULATOR_URL}/v1`,
  });
  
  console.log('Testing chat completion streaming...');
  
  const stream = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [
      { role: 'user', content: 'Count to 3' }
    ],
    stream: true,
  });
  
  const chunks = [];
  let content = '';
  
  console.log('Reading stream...');
  for await (const chunk of stream) {
    chunks.push(chunk);
    console.log('Got chunk:', JSON.stringify(chunk));
    
    if (chunk.choices && chunk.choices.length > 0) {
      const choice = chunk.choices[0];
      if (choice.delta?.content) {
        content += choice.delta.content;
      }
    }
  }
  
  console.log(`\nTotal chunks: ${chunks.length}`);
  console.log(`Content: "${content}"`);
  
  if (chunks.length > 0) {
    console.log('✓ Test PASSED');
  } else {
    console.log('✗ Test FAILED - No chunks received');
  }
}

// Run
await loadTestScripts();
await runTest();