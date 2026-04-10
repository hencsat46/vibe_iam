FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source + already-generated proto stubs
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# ─── Runtime image ───────────────────────────────────────────────
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /server /server

EXPOSE 50051
ENTRYPOINT ["/server"]
