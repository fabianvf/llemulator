#!/bin/bash

# Script to run tests with the emulator

set -e

echo "🚀 Starting OpenAI Emulator..."

# Start emulator in background
cd ..
./openai-emulator &
EMULATOR_PID=$!

# Wait for emulator to be ready
echo "⏳ Waiting for emulator to be ready..."
for i in {1..30}; do
  if curl -s http://localhost:8080/healthz > /dev/null; then
    echo "✅ Emulator is ready!"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "❌ Emulator failed to start"
    kill $EMULATOR_PID 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

# Run tests
echo "🧪 Running tests..."
cd example-app
npm test
TEST_RESULT=$?

# Cleanup
echo "🧹 Cleaning up..."
kill $EMULATOR_PID 2>/dev/null || true

# Exit with test result
if [ $TEST_RESULT -eq 0 ]; then
  echo "✅ All tests passed!"
else
  echo "❌ Some tests failed"
fi

exit $TEST_RESULT