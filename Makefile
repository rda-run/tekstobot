# Load environment variables from .env to use with psql
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DB_URL=postgresql://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

export VERSION=$(shell cat .version)
LDFLAGS=-X main.Version=$(VERSION)

.PHONY: help build run release package whisper whisper-stop whisper-cpu clean check migrate-up migrate-down

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "Bot Makefile"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# Build the main binary
build: check ## Build the main binary
	@go build -ldflags "$(LDFLAGS)" -o bin/tekstobot cmd/tekstobot/main.go

# Run the backend locally
run: check ## Run the backend locally
	@go run -ldflags "$(LDFLAGS)" cmd/tekstobot/main.go

# Optimized build for production
release: check ## Build the optimized production binary
	@CGO_ENABLED=0 go build -ldflags "-s -w $(LDFLAGS)" -trimpath -o bin/tekstobot cmd/tekstobot/main.go

# Package into RPM using nfpm
package: release ## Create RPM package using nfpm
	@VERSION=$(VERSION) nfpm pkg --packager rpm --target .

# Strict rule required: build check guaranteeing no unused imports or syntax regressions
check: ## Strict build check
	@go build -o /dev/null ./...

# Boot Whisper small container on GPU via podman
whisper: ## Boot Whisper container (GPU via podman)
	@mkdir -p ~/.cache/huggingface
	@podman run -d --name faster-whisper-server \
		--device nvidia.com/gpu=all \
		--security-opt=label=disable \
		-p 8000:8000 \
		-e MODEL_NAME=medium \
		-v ~/.cache/huggingface:/root/.cache/huggingface \
		fedirz/faster-whisper-server:latest-cuda

# Destroy Whisper container
whisper-stop: ## Destroy Whisper container
	@podman stop faster-whisper-server && podman rm faster-whisper-server

# Fallback target running on CPU only (for systems without GPU support or CDI)
whisper-cpu: ## Boot Whisper container in CPU mode
	@mkdir -p ~/.cache/huggingface
	@podman run -d --name faster-whisper-server \
		--security-opt=label=disable \
		-p 8000:8000 \
		-e MODEL_NAME=medium \
		-v ~/.cache/huggingface:/root/.cache/huggingface \
		fedirz/faster-whisper-server:latest-cpu

clean: ## Remove bin directory and media data
	@rm -rf bin/tekstobot data/media

# Database Migrations targets using psql
migrate-up: ## Execute UP migrations on PostgreSQL
	@echo "Executing UP migrations on PostgreSQL..."
	@for file in $$(ls internal/db/migrations/*.up.sql | sort); do \
		echo "Applying $$file..."; \
		psql "$(DB_URL)" -f "$$file"; \
	done

migrate-down: ## Execute DOWN migrations on PostgreSQL (Rollback)
	@echo "Executing DOWN migrations on PostgreSQL (Rollback)..."
	@for file in $$(ls internal/db/migrations/*.down.sql | sort -r); do \
		echo "Rolling back $$file..."; \
		psql "$(DB_URL)" -f "$$file"; \
	done
