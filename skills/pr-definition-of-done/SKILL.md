---
name: pr-definition-of-done
description: >-
  Definition of done checklist for dora-metrics pull requests. Use when
  preparing a PR, reviewing code, or when the user asks if their changes are
  ready to merge.
---

# PR Definition of Done

## Checklist

Before marking a PR as ready for review, verify:

### Code Quality
- [ ] `make test` passes locally (all existing tests green)
- [ ] `make lint` passes with no new warnings
- [ ] `go mod tidy` has been run (go.sum is consistent)
- [ ] No `fmt.Println` or debug logging left in code
- [ ] Errors are wrapped with context (`fmt.Errorf("doing X: %w", err)`)
- [ ] New code uses `pkg/logger` for all output

### Testing
- [ ] New functionality has tests (table-driven, using `testify/assert`)
- [ ] Tests don't require external services (mock Redis, GitHub, DevLake)
- [ ] `go test -race ./...` shows no race conditions

### Configuration
- [ ] New config fields added to both `internal/config/types.go` AND `configs/config.yaml`
- [ ] Environment variables documented in README if user-facing
- [ ] Sensitive values use env vars, not config file literals

### Deployment
- [ ] If new env vars required: update `manifests/local/secrets.yaml` template
- [ ] If new RBAC needed: update `manifests/local/rbac.yaml`
- [ ] Dockerfile still builds: `docker build -t dora-metrics .`

### Documentation
- [ ] README updated if adding user-visible features or config
- [ ] Architecture docs updated if changing data flow (see `docs/`)
- [ ] AGENTS.md still under 60 lines if modified

## CI/CD Quirks

- **GitHub Actions**: Runs `go test -race` with coverage upload to Codecov.
  Tests must pass without Redis, ArgoCD, or any external service.
- **Tekton/Konflux**: Builds container image on every PR via `.tekton/`
  pipelines. Image is tagged with PR revision and expires after 5 days.
- **No e2e tests in CI**: Integration with ArgoCD/DevLake is only verified
  in staging. Unit tests must mock all external dependencies.

## Commit Message Convention

Follow conventional commits (see git log for examples):
```
feat(argocd): add support for multi-cluster routing
fix(webrca): correct date parsing for incident resolution
chore(deps): update module golang.org/x/oauth2 to v0.36.0
```
