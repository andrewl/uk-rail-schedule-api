SYNCD_BIN := bin/syncd
WEB_BIN   := bin/web

ifneq (,$(wildcard .env))
    include .env
    export
endif

build:
	go build -o $(SYNCD_BIN) ./cmd/syncd
	go build -o $(WEB_BIN)   ./cmd/web

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

.PHONY: build test run dockerise dev
