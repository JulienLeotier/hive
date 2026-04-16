# Story 7.9: CI/CD Pipeline & Cross-Platform Build

Status: done

## Story

As a contributor,
I want automated CI/CD that tests, builds, and releases Hive for all platforms,
so that every merge produces a verified, cross-platform binary.

## Acceptance Criteria

1. **Given** a pull request is opened against the main branch
   **When** GitHub Actions CI runs
   **Then** it executes `go vet`, `golangci-lint`, and the full test suite
   **And** CI fails if any check fails or test coverage drops below 80% on core packages
   **And** CI completes in under 5 minutes

2. **Given** a new tag is pushed (e.g., `v0.1.0`)
   **When** the release workflow triggers
   **Then** GoReleaser builds binaries for macOS (arm64, x64), Linux (x64, arm64), Windows (x64)
   **And** creates a GitHub Release with checksums and changelog
   **And** updates the Homebrew tap formula
   **And** builds and pushes the Docker image (~15MB scratch-based)

## Tasks / Subtasks

- [x] Task 1: GitHub Actions CI workflow (AC: #1)
  - [x] Create `.github/workflows/ci.yml` triggered on pull requests and main branch pushes
  - [x] Steps: checkout, setup Go 1.24+, `go vet ./...`, `golangci-lint run`, `go test ./... -coverprofile=coverage.out`
  - [x] Fail if coverage on core packages (internal/task, internal/event, internal/resilience) drops below 80%
  - [x] Cache Go modules and build cache for speed
  - [x] Target: CI completes in under 5 minutes
- [x] Task 2: GoReleaser configuration (AC: #2)
  - [x] Create `.goreleaser.yaml` with build targets: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64
  - [x] Configure ldflags for version injection: `-X github.com/JulienLeotier/hive/internal/cli.Version={{.Version}}`
  - [x] Enable checksum generation (SHA256)
  - [x] Enable changelog generation from conventional commits
  - [x] Configure archive format: tar.gz for Linux/macOS, zip for Windows
- [x] Task 3: Release workflow (AC: #2)
  - [x] Create `.github/workflows/release.yml` triggered on tag push (`v*`)
  - [x] Steps: checkout, setup Go, run GoReleaser with `GITHUB_TOKEN`
  - [x] GoReleaser creates GitHub Release with binaries, checksums, and changelog
- [x] Task 4: Homebrew tap (AC: #2)
  - [x] Configure GoReleaser to generate Homebrew formula
  - [x] Formula points to the GitHub Release binaries
- [x] Task 5: Docker image (AC: #2)
  - [x] Create `Dockerfile` with multi-stage build: Go builder stage + scratch final stage
  - [x] Final image contains only the hive binary (~15MB)
  - [x] Configure GoReleaser Docker builds or add to release workflow
- [x] Task 6: Makefile targets (AC: #1)
  - [x] Verify `make test` runs full test suite
  - [x] Verify `make lint` runs `go vet`
  - [x] Existing Makefile targets support CI pipeline

## Dev Notes

### Architecture Compliance

- Single binary, cross-platform builds via GoReleaser (NFR13)
- Zero external dependencies at runtime — pure Go with modernc.org/sqlite (NFR14)
- Docker image uses `scratch` base for minimal attack surface (~15MB)
- Version injection via ldflags matches the existing `internal/cli.Version` variable
- Go module cache in CI reduces build times to under 5 minutes

### Key Design Decisions

- GoReleaser handles all cross-compilation, checksums, and release creation — single tool for the entire release pipeline
- CI runs on every PR (not just main) to catch issues before merge
- Coverage threshold is 80% on core packages — critical business logic must be well-tested
- Docker image is scratch-based (no shell, no OS) for security and size
- Homebrew tap enables `brew install hive` for macOS users

### CI Pipeline Flow

```
PR opened -> checkout -> setup Go -> go vet -> golangci-lint -> go test (coverage) -> pass/fail
Tag pushed -> checkout -> setup Go -> GoReleaser (build + release + Docker + Homebrew)
```

### Build Targets

| OS | Arch | Format |
|---|---|---|
| macOS | arm64 | tar.gz |
| macOS | amd64 | tar.gz |
| Linux | amd64 | tar.gz |
| Linux | arm64 | tar.gz |
| Windows | amd64 | zip |

### Integration Points

- `.github/workflows/ci.yml` — CI pipeline
- `.github/workflows/release.yml` — release pipeline
- `.goreleaser.yaml` — GoReleaser configuration
- `Dockerfile` — Docker image build
- `Makefile` — build, test, lint targets
- `internal/cli/version.go` — `Version` variable injected via ldflags

### References

- [Source: _bmad-output/planning-artifacts/architecture.md#Build & Deployment]
- [Source: _bmad-output/planning-artifacts/prd.md#NFR13, NFR14]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- GitHub Actions CI workflow with go vet, golangci-lint, and test coverage enforcement
- GoReleaser configured for 5 platform targets with checksum and changelog generation
- Release workflow triggered by tag push, creates GitHub Release with all artifacts
- Scratch-based Docker image at ~15MB for minimal footprint
- Homebrew tap formula generation for macOS distribution
- CI targets under 5 minutes with Go module caching

### Change Log

- 2026-04-16: Story 7.9 implemented — CI/CD pipeline with cross-platform GoReleaser builds

### File List

- .github/workflows/ci.yml (new)
- .github/workflows/release.yml (new)
- .goreleaser.yaml (new)
- Dockerfile (new)
- Makefile (reference — build, test, lint targets)
- internal/cli/version.go (reference — Version variable for ldflags injection)
