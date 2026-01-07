#!/bin/bash

# Test runner script for OpenAI emulator
# This script demonstrates how to properly test against the emulator

set -e

echo "Starting OpenAI emulator..."
./openai-emulator &
EMULATOR_PID=$!

# Give the server time to start
sleep 2

# Check if server is ready
if ! curl -f -s http://localhost:8080/healthz > /dev/null; then
    echo "ERROR: Server failed to start"
    kill $EMULATOR_PID 2>/dev/null
    exit 1
fi

echo "Server started successfully (PID: $EMULATOR_PID)"

# Function to cleanup on exit
cleanup() {
    echo "Stopping emulator..."
    kill $EMULATOR_PID 2>/dev/null || true
}
trap cleanup EXIT

echo ""
echo "Running Go unit tests..."
go test -v ./internal/...

echo ""
echo "Running Go integration tests..."
EMULATOR_URL=http://localhost:8080 go test -v ./tests/...

echo ""
echo "Running JavaScript conformance tests..."
(cd conformance/js && npm test)

echo ""
echo "Running Python conformance tests..."
(cd conformance/python && python -m pytest test_conformance.py -v)

echo ""
echo "All tests passed!"