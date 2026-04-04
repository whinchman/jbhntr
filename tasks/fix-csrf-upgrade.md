# Task: fix-csrf-upgrade

**Type:** coder
**Status:** pending
**Priority:** 1
**Epic:** none
**Depends On:** none

## Description

The Render build is failing because `csrf.Exempt` is called in `internal/web/server.go:365` but does not exist in `github.com/gorilla/csrf v1.7.2`. Upgrade the package to its latest version, which includes this function.

## Acceptance Criteria

- [ ] `go get github.com/gorilla/csrf@latest` is run and `go.mod`/`go.sum` are updated
- [ ] `go mod tidy` is run
- [ ] `go build ./cmd/jobhuntr` exits 0 with no errors
- [ ] Changes are committed to the `development` branch

## Context

- Failing deploy: `dep-d782m4nafjfc73ff07gg` on service `srv-d77ve595pdvs739gqtf0`
- Error: `internal/web/server.go:365:26: undefined: csrf.Exempt`
- No code changes needed — only `go.mod` and `go.sum` should change
- Repo root: `/home/whinchman/experiments/jobhuntr`

## Notes

