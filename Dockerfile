# Multi-stage build. The builder layer compiles the Hive binary from source;
# the runtime layer is distroless + non-root. Suitable for both local
# docker-compose flows (build-on-pull) and GoReleaser-shaped CI (which
# skips the builder and COPYs a prebuilt binary into the same runtime).
FROM golang:1.26-alpine AS builder
WORKDIR /src
# Cache dependencies first so source edits don't bust the module layer.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# -trimpath + reproducible embedded metadata keeps binaries diffable across
# rebuilds; -s -w strips debug info for size. Static link via CGO=0 so the
# distroless layer can run it without libc.
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/hive ./cmd/hive

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/hive /usr/local/bin/hive

# Default Hive listens on 8233 (see internal/config.Default). Override with
# HIVE_PORT or a cfg.Port in hive.yaml mounted at /data/hive.yaml.
EXPOSE 8233
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/hive"]
CMD ["serve"]
