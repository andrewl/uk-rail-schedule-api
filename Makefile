SYNCD_BIN := bin/syncd
WEB_BIN   := bin/web

VERSION    ?= dev
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X 'main.version=$(VERSION) $(BUILD_TIME)'

ifneq (,$(wildcard .env))
    include .env
    export
endif

build:
	go build -ldflags="$(LDFLAGS)" -o $(SYNCD_BIN) ./cmd/syncd
	go build -ldflags="$(LDFLAGS)" -o $(WEB_BIN)   ./cmd/web

test:
	go test ./...

run:
	./$(SYNCD_BIN) &
	./$(WEB_BIN)

dockerise:
	@echo "Building the docker image as $(DOCKER_IMAGE_NAME)"
	docker build -f Dockerfile -t $(shell basename $(shell pwd)):$(shell date +%Y%m%d%H%M%S) .

dev:
	docker compose up --build

update-feed:
	./update-schedule-feed.sh

dev-update-feed:
	docker compose run --rm update-feed

.PHONY: build test run dockerise dev update-feed dev-update-feed
