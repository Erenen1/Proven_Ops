.PHONY: help build test test-unit test-race test-go test-py lab-up lab-down validate-production clean

help:
	@echo "ProvenOps Makefile Targets:"
	@echo "  make build               - Compile Control Plane and Agent binaries"
	@echo "  make test                - Run all unit tests across Go, Python, and Dashboard"
	@echo "  make test-unit           - Run Go and Python unit tests"
	@echo "  make test-race           - Run Go tests with race detector enabled"
	@echo "  make lab-up              - Launch complete 5-node Docker lab environment"
	@echo "  make lab-down            - Stop and tear down the Docker lab environment"
	@echo "  make validate-production - Run the empirical production validation suite"
	@echo "  make clean               - Remove build artifacts and temporary files"

build:
	mkdir -p bin
	cd apps/control-plane && go build -ldflags="-s -w" -o ../../bin/control-plane ./cmd/server
	cd agent && go build -ldflags="-s -w" -o ../bin/provenops-agent ./cmd/agent

test-unit:
	cd apps/control-plane && go test -v -count=1 ./...
	cd agent && go test -v -count=1 ./...
	cd apps/ai-service && pytest

test-race:
	cd apps/control-plane && go test -v -race ./...
	cd agent && go test -v -race ./...

test-go:
	cd apps/control-plane && go test -v ./...
	cd agent && go test -v ./...

test-py:
	cd apps/ai-service && pytest

test: test-unit

lab-up:
	docker compose --profile lab up --build -d

lab-down:
	docker compose --profile lab down -v --remove-orphans

validate-production:
	python scripts/validate_production.py

clean:
	rm -rf bin/ dist/ apps/dashboard/dist/
