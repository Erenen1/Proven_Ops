.PHONY: help dev up down proto test-go test-py test lint build-agent run-agent

help:
	@echo "OpsPilot Development Makefile"
	@echo "  make dev         - Start central stack (Postgres, Control Plane, AI Service, Dashboard)"
	@echo "  make down        - Tear down all containers"
	@echo "  make proto       - Compile protobuf definitions for Go and Python"
	@echo "  make test        - Run all tests across Go, Python, and Dashboard"
	@echo "  make build-agent - Compile the Server Agent for Linux amd64"

dev:
	docker compose up --build -d

down:
	docker compose down

test-go:
	cd apps/control-plane && go test -v ./...
	cd agent && go test -v ./...

test-py:
	cd apps/ai-service && pytest

test: test-go test-py

build-agent:
	cd agent && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/opspilot-agent ./cmd/agent
