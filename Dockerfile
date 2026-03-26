# --- Build stage ---
FROM --platform=$BUILDPLATFORM golang:1.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o app .

# --- Runtime stage ---
FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/app .

# Environment variable placeholders
ENV API_USERNAME=""
ENV API_PASSWORD=""

EXPOSE 8080
CMD ["./app"]
