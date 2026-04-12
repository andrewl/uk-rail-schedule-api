# --- Build stage ---
# Debian-based builder is required for mattn/go-sqlite3 (CGO + glibc)
FROM golang:1.23 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends gcc libc-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/syncd ./cmd/syncd && \
    go build -o bin/web   ./cmd/web

# --- Runtime stage ---
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/bin/syncd ./syncd
COPY --from=builder /app/bin/web   ./web

EXPOSE 3333
CMD ["./web"]
