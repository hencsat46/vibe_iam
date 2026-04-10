.PHONY: proto build run test lint docker-up docker-down

# ─── Proto generation ───────────────────────────────────────────
# Requires: buf (https://buf.build/docs/installation)
proto:
	buf generate proto

# ─── Build ──────────────────────────────────────────────────────
build: proto
	go build -o bin/server ./cmd/server

# ─── Run (local, requires postgres running) ─────────────────────
run: build
	./bin/server

# ─── Docker ─────────────────────────────────────────────────────
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

# ─── Lint ───────────────────────────────────────────────────────
lint:
	go vet ./...

# ─── Test ───────────────────────────────────────────────────────
test:
	go test ./... -count=1

# ─── gRPC UI (grpcurl examples) ─────────────────────────────────
# grpcurl -plaintext -d '{"system_id":"my-system","environment_name":"prod"}' \
#   localhost:50051 iam.v1.AccessObjectService/CreateAccessObject
