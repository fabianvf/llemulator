"""Tool call conformance tests for the OpenAI emulator."""

import json
import os

import httpx
import pytest
from openai import OpenAI

EMULATOR_URL = os.getenv('EMULATOR_URL', 'http://localhost:8080')
API_KEY = 'test-token-python-toolcalls'

# A rule that answers with a tool call, and a catch-all for the turn that
# follows once the client has sent the result back.
SCRIPT = {
    "reset": True,
    "rules": [
        {
            "pattern": ".*weather.*",
            "tool_calls": [
                {
                    "id": "call_1",
                    "type": "function",
                    "function": {
                        "name": "get_weather",
                        "arguments": '{"city":"Berlin"}',
                    },
                }
            ],
        },
        {"pattern": "", "response": "It is sunny in Berlin."},
    ],
}


@pytest.fixture
def client():
    response = httpx.post(
        f"{EMULATOR_URL}/_emulator/script",
        headers={"Authorization": f"Bearer {API_KEY}"},
        json=SCRIPT,
    )
    if response.status_code != 200:
        raise Exception(f"Failed to load script: {response.status_code} - {response.text}")

    return OpenAI(api_key=API_KEY, base_url=f"{EMULATOR_URL}/v1")


class TestToolCalls:
    def test_returns_a_tool_call(self, client):
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "what is the weather"}],
        )

        assert completion.choices[0].finish_reason == "tool_calls"
        calls = completion.choices[0].message.tool_calls
        assert len(calls) == 1
        assert calls[0].function.name == "get_weather"
        assert json.loads(calls[0].function.arguments) == {"city": "Berlin"}

    def test_accumulates_a_streamed_tool_call(self, client):
        """The SDK reassembles streamed calls by index, and raises without one."""
        with client.chat.completions.stream(
            model='gpt-4',
            messages=[{"role": "user", "content": "what is the weather"}],
        ) as stream:
            completion = stream.get_final_completion()

        assert completion.choices[0].finish_reason == "tool_calls"
        calls = completion.choices[0].message.tool_calls
        assert len(calls) == 1, "the SDK dropped the streamed tool call"
        assert calls[0].function.name == "get_weather"
        assert json.loads(calls[0].function.arguments) == {"city": "Berlin"}

    def test_accepts_the_tool_result(self, client):
        first = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "what is the weather"}],
        )
        call = first.choices[0].message.tool_calls[0]

        second = client.chat.completions.create(
            model='gpt-4',
            messages=[
                {"role": "user", "content": "what is the weather"},
                first.choices[0].message,
                {"role": "tool", "tool_call_id": call.id, "content": '{"tempC":21}'},
            ],
        )

        assert second.choices[0].finish_reason == "stop"
        assert second.choices[0].message.content == "It is sunny in Berlin."

    def test_text_replies_carry_no_tool_calls(self, client):
        completion = client.chat.completions.create(
            model='gpt-4',
            messages=[{"role": "user", "content": "just say something"}],
        )

        assert completion.choices[0].finish_reason == "stop"
        assert completion.choices[0].message.tool_calls is None
