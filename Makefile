SYNCD_BIN := bin/syncd
WEB_BIN   := bin/web

build:
	go build -o $(SYNCD_BIN) ./cmd/syncd
	go build -o $(WEB_BIN)   ./cmd/web

test:
	go test ./...

dockerise:
	@echo "Building the docker image as $(DOCKER_IMAGE_NAME)"
	docker build -f Dockerfile -t $(shell basename $(shell pwd)):$(shell date +%Y%m%d%H%M%S) .

.PHONY: build test dockerise
