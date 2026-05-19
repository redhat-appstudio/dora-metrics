---
name: run-tests
description: >-
  Run unit tests and verify code quality for dora-metrics. Use when the user
  asks to run tests, check coverage, validate changes, or verify a fix.
---

# Run Tests

## Unit Tests

```bash
# Run all tests (fast, no external deps)
make test

# Run tests with race detection (what CI does)
go test ./... -v -race

# Run with coverage report
make unit-test
# View HTML coverage: go tool cover -html=coverage.out
```

## Single-Package Tests

```bash
# Test a specific package
go test -v ./pkg/monitors/webrca/...
go test -v ./pkg/integrations/...
go test -v ./pkg/auth/...

# Test with verbose output and race detection
go test -v -race -count=1 ./pkg/monitors/webrca/...
```

## Lint

Requires `golangci-lint` installed (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6`).

```bash
# Full lint
make lint

# Single file
golangci-lint run ./path/to/file.go

# Auto-fix
golangci-lint run --fix ./...
```

## Test Coverage Gaps

The following packages have tests:
- `pkg/monitors/webrca/` (client, monitor, types, incidents, integration)
- `pkg/integrations/` (devlake)
- `pkg/auth/` (token)
- `pkg/logger/`
- `internal/version/`

The following packages currently lack tests:
- `pkg/monitors/argocd/` (largest package — be cautious modifying)
- `pkg/storage/`
- `internal/config/`
- `internal/server/`

## Pre-Commit Hooks

```bash
# Install hooks (one-time)
pre-commit install

# Run hooks manually against all files
pre-commit run --all-files
```
