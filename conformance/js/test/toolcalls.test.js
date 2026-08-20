import { describe, it, beforeEach } from 'node:test';
import assert from 'node:assert';
import OpenAI from 'openai';

const EMULATOR_URL = process.env.EMULATOR_URL || 'http://localhost:8080';
const API_KEY = 'test-token-js-toolcalls';

// A rule that answers with a tool call, and a catch-all for the turn that
// follows once the client has sent the result back.
const SCRIPT = {
  reset: true,
  rules: [
    {
      pattern: '.*weather.*',
      tool_calls: [
        {
          id: 'call_1',
          type: 'function',
          function: { name: 'get_weather', arguments: '{"city":"Berlin"}' },
        },
      ],
    },
    { pattern: '', response: 'It is sunny in Berlin.' },
  ],
};

async function loadScript() {
  const response = await fetch(`${EMULATOR_URL}/_emulator/script`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(SCRIPT),
  });
  if (!response.ok) {
    throw new Error(`Failed to load script: ${response.status} - ${await response.text()}`);
  }
}

describe('OpenAI Emulator - Tool Calls', () => {
  let openai;

  beforeEach(async () => {
    openai = new OpenAI({ apiKey: API_KEY, baseURL: `${EMULATOR_URL}/v1` });
    await loadScript();
  });

  it('returns a tool call the SDK can read', async () => {
    const completion = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'what is the weather' }],
    });

    assert.equal(completion.choices[0].finish_reason, 'tool_calls');
    const calls = completion.choices[0].message.tool_calls;
    assert.equal(calls.length, 1);
    assert.equal(calls[0].function.name, 'get_weather');
    assert.deepEqual(JSON.parse(calls[0].function.arguments), { city: 'Berlin' });
  });

  // The streamed shape is what the SDK reassembles, and it drops any call that
  // arrives without an index rather than failing, so assert on the accumulated
  // message rather than on the raw chunks.
  it('accumulates a streamed tool call', async () => {
    const stream = openai.beta.chat.completions.stream({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'what is the weather' }],
    });
    const completion = await stream.finalChatCompletion();

    assert.equal(completion.choices[0].finish_reason, 'tool_calls');
    const calls = completion.choices[0].message.tool_calls;
    assert.equal(calls.length, 1, 'the SDK dropped the streamed tool call');
    assert.equal(calls[0].function.name, 'get_weather');
    assert.deepEqual(JSON.parse(calls[0].function.arguments), { city: 'Berlin' });
  });

  it('accepts the tool result and continues the conversation', async () => {
    const first = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'what is the weather' }],
    });
    const call = first.choices[0].message.tool_calls[0];

    const second = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [
        { role: 'user', content: 'what is the weather' },
        first.choices[0].message,
        { role: 'tool', tool_call_id: call.id, content: '{"tempC":21}' },
      ],
    });

    assert.equal(second.choices[0].finish_reason, 'stop');
    assert.equal(second.choices[0].message.content, 'It is sunny in Berlin.');
  });

  it('leaves text replies without a tool_calls field', async () => {
    const completion = await openai.chat.completions.create({
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'just say something' }],
    });

    assert.equal(completion.choices[0].finish_reason, 'stop');
    assert.equal(completion.choices[0].message.tool_calls, undefined);
  });
});
