.PHONY: dev dev-down db-up dev-native build run test test-race clean

# Start full dev stack (Docker + hot-reload via air)
dev:
	docker compose --profile dev up --build

# Stop the dev stack
dev-down:
	docker compose --profile dev down

# Start only the database service
db-up:
	docker compose up -d db

# Run the app natively (requires DATABASE_URL to be set)
dev-native:
	air

# Build the binary
build:
	go build -o bin/jobhuntr ./cmd/jobhuntr

# Run the app using the pre-built binary
run:
	./run.sh

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Remove build artifacts
clean:
	rm -rf bin/ tmp/ output/*.html output/*.pdf output/*.docx
