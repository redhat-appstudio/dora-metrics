# AGENTS.md — DORA Metrics Server

Go service that watches ArgoCD deployments and WebRCA incidents, then forwards structured events to Apache DevLake for DORA metrics calculation (deployment frequency, lead time, change failure rate, MTTR).

## Build, Test & Verify

```bash
make build          # binary → bin/dora-metrics
make test           # go test ./... (no external deps needed)
make unit-test      # same + coverage report
make lint           # golangci-lint run
# Single-file lint:
golangci-lint run ./path/to/file.go
# Single-file type-check:
staticcheck ./path/to/package/...
```

CI runs `go test ./... -v -race` on every PR (`.github/workflows/tests.yml`).
Tekton pipelines in `.tekton/` handle container builds via Konflux.

## Code Layout

- `cmd/server/` — entrypoint, CLI flags
- `internal/config/` — YAML config loading (`configs/config.yaml`)
- `internal/server/` — Fiber HTTP server bootstrap
- `pkg/monitors/argocd/` — ArgoCD watcher, event/image/commit processors
- `pkg/monitors/webrca/` — WebRCA incident polling
- `pkg/integrations/` — DevLake webhook client
- `pkg/storage/` — Redis-backed deployment cache
- `pkg/auth/` — OCM token refresh
- `manifests/` — Kubernetes/OpenShift deployment (kustomize)

## Conventions

- Logging: use `pkg/logger` (zap-based); follow `.cursor/logging_standards`
- Config: add new fields to `internal/config/types.go` + `configs/config.yaml`
- Errors: wrap with context; never silently swallow; log at call site
- Tests: table-driven, use `testify/assert`; place in `*_test.go` beside source
- Deps: run `go mod tidy` after changes; prefer stdlib over new dependencies
- Architecture docs: [architecture](docs/architecture.md), [data flow](docs/data-flow.md), [deployment](docs/deployment-guide.md)

## Don't

- Don't add dependencies without justification — binary runs in constrained pods
- Don't commit secrets; env vars `OFFLINE_TOKEN` and `DEVLAKE_WEBHOOK_TOKEN` are required at runtime only
- Don't modify `configs/config.yaml` team routing without coordinating with team leads
- Don't use `fmt.Println` for logging — always use `pkg/logger`
- Don't skip `go mod tidy` — CI will fail on inconsistent go.sum

## Pattern References

- New monitor: follow `pkg/monitors/webrca/` (client + monitor + types pattern)
- New integration: follow `pkg/integrations/devlake.go` (HTTP client + types)
- Add a team to DevLake routing: add entry in `configs/config.yaml` under `teams:`
- New API endpoint: follow `apis/health/` (handler + types + route registration)

## Review Cadence
Reviewed quarterly (next: Q3 2026). Update when adding packages, build steps, or structure.
