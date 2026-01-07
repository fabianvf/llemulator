.PHONY: all test conformance build run docker-build docker-run clean

# Variables
BINARY_NAME=openai-emulator
DOCKER_IMAGE=openai-emulator
PORT?=8080

all: test build

# Build the Go binary
build:
	go build -o $(BINARY_NAME) cmd/openai-emulator/main.go

# Run the server locally
run: build
	./$(BINARY_NAME)

# Run Go tests
test-go:
	go test -v ./...

# Install JS dependencies
install-js:
	cd conformance/js && npm install

# Install Python dependencies  
install-python:
	cd conformance/python && pip install -r requirements.txt

# Run JS conformance tests
test-js: install-js
	cd conformance/js && npm test

# Run Python conformance tests
test-python: install-python
	cd conformance/python && pytest -v

# Run conformance tests (both JS and Python)
conformance: test-js test-python

# Run all tests (Go unit tests + conformance)
test: test-go conformance

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run: docker-build
	docker run -p $(PORT):8080 -e DEBUG=true $(DOCKER_IMAGE)

# Run conformance tests against Docker container
docker-test: docker-build
	@echo "Starting container..."
	@docker run -d --name emulator-test -p $(PORT):8080 $(DOCKER_IMAGE)
	@sleep 2
	@echo "Running conformance tests..."
	@EMULATOR_URL=http://localhost:$(PORT) make conformance || (docker stop emulator-test && docker rm emulator-test && exit 1)
	@echo "Stopping container..."
	@docker stop emulator-test
	@docker rm emulator-test
	@echo "Tests passed!"

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf conformance/js/node_modules
	rm -rf conformance/python/__pycache__
	rm -rf conformance/python/.pytest_cache

# Help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the Go binary"
	@echo "  make run           - Run the server locally"
	@echo "  make test          - Run all tests (Go + conformance)"
	@echo "  make test-go       - Run Go unit tests"
	@echo "  make conformance   - Run conformance tests (JS + Python)"
	@echo "  make test-js       - Run JavaScript conformance tests"
	@echo "  make test-python   - Run Python conformance tests"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container"
	@echo "  make docker-test   - Run conformance tests against Docker"
	@echo "  make clean         - Clean build artifacts"