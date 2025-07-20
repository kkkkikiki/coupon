.PHONY: help build run test clean docker-up docker-down proto-gen

# Default target
help:
	@echo "Available commands:"
	@echo "  build        - Build the coupon server"
	@echo "  run          - Run the coupon server"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-up    - Start PostgreSQL and Redis with Docker Compose"
	@echo "  docker-down  - Stop Docker Compose services"
	@echo "  proto-gen    - Generate protobuf code"
	@echo "  setup        - Setup development environment"

# Build the application
build:
	go build -o bin/coupon-server ./cmd

# Run the application
run: build
	./bin/coupon-server

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf gen/

# Start Docker services
docker-up:
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Services are ready!"
	@echo "PostgreSQL: localhost:5432"
	@echo "Redis: localhost:6379"
	@echo "Redis Commander: http://localhost:8081"

# Stop Docker services
docker-down:
	docker-compose down

# Generate protobuf code
proto-gen:
	$(shell go env GOPATH)/bin/buf generate

# Setup development environment
setup:
	@echo "Setting up development environment..."
	go mod tidy
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
	@echo "Copying environment file..."
	cp .env.example .env
	@echo "Setup complete!"
	@echo "1. Edit .env file with your configuration"
	@echo "2. Run 'make docker-up' to start databases"
	@echo "3. Run 'make run' to start the server"

# Development workflow
dev: docker-up
	@echo "Starting development server..."
	@sleep 5
	make run

# Stop development environment
dev-stop: docker-down
