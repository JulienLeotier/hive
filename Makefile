VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/JulienLeotier/hive/internal/cli.Version=$(VERSION)"

AIR ?= $(shell command -v air 2>/dev/null)
GOBIN := $(shell go env GOPATH)/bin

.PHONY: build test lint dev dev-api dev-web clean dashboard air-install

dashboard:
	cd web && npm run build

build: dashboard
	go build $(LDFLAGS) -o hive ./cmd/hive

test:
	go test ./... -v -count=1

lint:
	go vet ./...

# Full-stack hot-reload dev.
#   - Vite dev server on :5173 with HMR for .svelte / .ts
#   - Go server on :8233 via air (rebuild on .go change)
#   - Vite proxies /api and /ws to the Go server
# Open http://localhost:5173
dev: air-install
	@echo "▶ http://localhost:5173  (Vite HMR · proxies /api + /ws to Go :8233)"
	@trap 'kill 0' INT TERM EXIT; \
		( $(or $(AIR),$(GOBIN)/air) -c .air.toml ) & \
		( cd web && npm install --silent && npm run dev ) & \
		wait

dev-api: air-install
	@$(or $(AIR),$(GOBIN)/air) -c .air.toml

dev-web:
	cd web && npm install --silent && npm run dev

air-install:
	@if [ -z "$(AIR)" ] && [ ! -x "$(GOBIN)/air" ]; then \
		echo "▶ installing air (Go live-reload)..."; \
		go install github.com/air-verse/air@latest; \
	fi

serve: build
	./hive serve

clean:
	rm -f hive
	rm -rf internal/dashboard/dist .air-tmp build-errors.log
