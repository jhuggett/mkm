# Compile the API server binary
.PHONY: build
build:
	@echo "Building API service..."

# Run the test suite with coverage
.PHONY: test
test: build
	@echo "Running API tests..."

# Deploy the API to production
.PHONY: deploy
deploy: build test
	@echo "Deploying API service..."