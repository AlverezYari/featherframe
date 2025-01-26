.PHONY: build run clean

# Build the application
build:
	go build -o bin/featherframe cmd/featherframe/main.go

# Run the application
run:
	go run cmd/featherframe/main.go

# Clean build artifacts
clean:
	rm -rf bin/

# Development helper to build and run
dev: build
	./bin/featherframe
