# ── Build Stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o neartap-server ./cmd/server/main.go

# ── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

# Install CA certificates for Firebase HTTPS calls
RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy built binary from builder
COPY --from=builder /app/neartap-server .
# Copy default environment template (if any) or local service keys if present
COPY --from=builder /app/.env.example .env

# Expose Go API port
EXPOSE 8080

CMD ["./neartap-server"]
