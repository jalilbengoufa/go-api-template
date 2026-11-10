# go-api-template

**Read [README.md](README.md) first.** It is the source of truth for the package layout, endpoints, response shapes, configuration variables, local setup, and `make` targets. Nothing in it is repeated here.

This file covers only what the README does not state: the conventions to follow and the decisions that look like mistakes but are not.

## Conventions

- **No comments in Go code**, including tests. The codebase is deliberately comment-free; express intent through naming instead. Do not add explanatory comments when editing.
- Go 1.27.3, pinned by the `go` directive. The system `go` may be older; `GOTOOLCHAIN=auto` fetches the right toolchain automatically, so plain `go build` works.
- Each layer declares the interface it consumes (`handler.ResourceService`, `service.ResourceRepository`) rather than importing the concrete type below it. Keep dependencies pointing inward, and wire everything explicitly in `cmd/api/main.go`.
- Handlers return errors; they never write an error response. Status-code mapping belongs in `internal/httpx.ErrorHandler`, and new failure modes belong there or as sentinels in `internal/domain`.
- Tests are table-driven, use fakes at the interface boundary, and assert with `t.Errorf`/`t.Fatalf` — no assertion library. Use `httptest.NewRequestWithContext(t.Context(), ...)`; the bare `httptest.NewRequest` fails lint.
- Configuration is read only in `internal/config`, never via `os.Getenv` elsewhere, and is validated once at startup so a bad environment fails fast.

## Verify before reporting work complete

```bash
make check    # tidy, vet, lint, test
```

Lint runs golangci-lint v2.15.0 with 44 linters and must report zero issues. Tests must pass under `-race`.

## Deliberate decisions — do not "fix" these

- `.golangci.yml` uses the golangci-lint **v2** schema (`version: "2"`, separate `formatters:` section). Do not convert it back to v1 keys. v1 cannot parse Go 1.27 and reports phantom `typecheck` errors.
- The `govulncheck` CI job is `continue-on-error: true` on purpose. Two echo v4 advisories (GO-2026-6875, GO-2026-6872) have **no fixed version**; govulncheck has no suppression mechanism, so leaving it blocking would stall every merge. Keep the job, and do not downgrade echo to silence it.
- Use `ContextTimeoutWithConfig`, never `TimeoutWithConfig` — the latter is deprecated upstream for data races.
- `e.IPExtractor` is `ExtractIPDirect()` unless `TRUSTED_PROXIES` is set, so `X-Forwarded-For` cannot spoof the rate-limiter key (CVE-2026-118653). Never widen this to `0.0.0.0/0`.
- A wrong method on an existing route returns **404, not 405**. Echo registers a catch-all when a route group carries middleware, which takes precedence over its own method-not-allowed response. Group-level auth was chosen over the stricter status code; changing it moves auth off the group.
- Schema changes go in `migrations/*.sql`. GORM `AutoMigrate` is intentionally unused so production changes stay reviewable and ordered.
- `internal/repository` and `internal/database` have no unit tests by design — they are only meaningfully tested against a real database, and no container-based test dependency has been added. Do not add one without asking.
- `Resource` is a placeholder entity, not a real domain concept. Replace or copy it as a set of four files (`domain`, `repository`, `service`, `handler`) and register the handler in `internal/server/server.go`.
