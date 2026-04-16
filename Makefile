VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/JulienLeotier/hive/internal/cli.Version=$(VERSION)"

.PHONY: build test lint dev clean dashboard

dashboard:
	cd web && npm run build

build: dashboard
	go build $(LDFLAGS) -o hive ./cmd/hive

test:
	go test ./... -v -count=1

lint:
	go vet ./...

dev:
	go run $(LDFLAGS) ./cmd/hive --log-level debug

serve: build
	./hive serve

clean:
	rm -f hive
	rm -rf internal/dashboard/dist
