# CLAUDE.md — DORA Metrics Server

## Purpose

This is a Go server deployed on OpenShift that automates DORA metrics collection
for Konflux (Red Hat's CI/CD platform). It watches ArgoCD application resources
via the Kubernetes watch API, detects deployment events (successful and failed),
extracts commit SHAs from container image tags, fetches commit metadata from
GitHub, and sends structured deployment payloads to Apache DevLake via webhooks.
It also polls WebRCA for incident data to compute Mean Time to Recovery.

## Stack

- Go 1.25+ (toolchain 1.26.2)
- gofiber/fiber v2 (HTTP framework)
- redis/go-redis v9 (deployment deduplication cache)
- argoproj/argo-cd v2 (Kubernetes types for ArgoCD Application CRD)
- google/go-github v53 (commit metadata retrieval)
- uber-go/zap (structured logging)
- stretchr/testify (test assertions)
- Container: UBI9-minimal base, built with Dockerfile (multi-stage)
- Deployment: OpenShift with kustomize (`manifests/local/` and `manifests/production/`)
- CI: GitHub Actions (`go test -race`) + Tekton/Konflux (container build)

## Code Layout

```
cmd/server/          → main.go + flags.go (entrypoint, CLI flag parsing)
internal/config/     → config.go, types.go, constants.go (YAML config loading)
internal/server/     → server.go (Fiber app setup, route registration)
internal/handlers/   → handlers.go (HTTP handler registration)
internal/version/    → version.go (build metadata injection via ldflags)
pkg/monitors/argocd/ → core logic: watcher, event processor, image/commit
                       processing, GitHub client, application parser
pkg/monitors/webrca/ → WebRCA incident polling and forwarding
pkg/integrations/    → DevLake webhook HTTP client + team routing
pkg/storage/         → Redis client for deployment record caching
pkg/auth/            → OCM offline token refresh for WebRCA API
pkg/logger/          → Zap logger wrapper with level configuration
apis/                → API route definitions (health, argocd endpoints)
configs/config.yaml  → runtime config (namespaces, teams, DevLake routing)
manifests/           → Kubernetes/OpenShift YAML (kustomize overlays)
```

## Build, Test, Run

```bash
# Build
make build                    # → bin/dora-metrics
go build -o bin/dora-metrics ./cmd/server

# Test (no external services required)
make test                     # go test ./...
make unit-test                # + coverage report → coverage.out
go test ./... -v -race        # what CI runs

# Lint
make lint                     # golangci-lint run

# Single-file lint:
golangci-lint run ./path/to/file.go
# Single-file type-check:
staticcheck ./path/to/package/...

# Run locally (requires Redis + env vars)
export OFFLINE_TOKEN="..."
export DEVLAKE_WEBHOOK_TOKEN="..."
make run

# Container
docker build -t dora-metrics .
```

## Design Choices

- **Event-driven, not polling for ArgoCD**: Uses Kubernetes watch API on
  ArgoCD Application CRDs for near-realtime deployment detection. WebRCA uses
  polling because its API doesn't support watches.
- **Redis for deduplication**: Prevents sending duplicate deployments to DevLake
  when the same commit deploys to multiple clusters. Key format:
  `dora-metrics:<component>:<revision>:<cluster>`.
- **Team routing**: Every deployment goes to a global DevLake project AND
  optionally to team-specific projects based on component→team mapping in config.
- **Image tag = commit SHA**: The system assumes container image tags are full
  Git commit SHAs. Non-SHA tags are filtered out.
- **Graceful degradation**: If GitHub API fails for a commit, that commit is
  skipped rather than blocking the entire deployment event.
- **No ORM or database**: All persistent state is in Redis with TTL-based expiry.
  The server is stateless and horizontally scalable.

## Pitfalls

- **ArgoCD monitor has no unit tests** — the `pkg/monitors/argocd/` package is
  the largest and most complex but currently untested. Be cautious modifying it.
- **Config struct drift**: `internal/config/types.go` must stay in sync with
  `configs/config.yaml` — there's no validation at startup beyond YAML parsing.
- **OCM token expiry**: The offline token for WebRCA silently expires; failures
  surface as 401 errors in logs, not crashes.
- **Commit date validation**: If a commit's authored date is in the future or
  unreasonably old, the deployment is skipped entirely (see `processor/validator.go`).
- **Repository blacklist**: `configs/config.yaml` contains a `repository_blacklist`
  that silently drops commits from listed repos — easy to miss during debugging.
- **No graceful shutdown**: The server doesn't drain in-flight events on SIGTERM;
  a deployment event may be lost during pod restarts.
- **`components_to_ignore` is a blocklist**: All components are monitored unless
  explicitly listed here. Adding a new ArgoCD app automatically gets monitored.
