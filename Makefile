GO ?= go
COMPOSE ?= docker compose

.PHONY: fmt test build run proto up down logs check

proto:
	./scripts/proto/generate.sh

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build:
	$(GO) build ./cmd/server

run:
	$(GO) run ./cmd/server

up:
	$(COMPOSE) up --build -d

down:
	$(COMPOSE) down --remove-orphans

logs:
	$(COMPOSE) logs -f server

check: proto fmt test build
