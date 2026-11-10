# go-api-template

A production-oriented starting point for JSON APIs in Go: Echo for routing, GORM over PostgreSQL, Auth0-issued JWT validation, `log/slog` structured logging, and a layered architecture wired through explicit dependency injection.

## Requirements

- Go 1.27+
- PostgreSQL 14+
- Docker (optional, for local dependencies)

## Quick start

```bash
cp .env.example .env
docker compose up -d postgres
make run
```

The API listens on `http://localhost:8080`. With `AUTH_DISABLED=true` every request is attributed to the synthetic subject `dev-user`, so endpoints can be exercised without an identity provider. Setting `AUTH_DISABLED=true` while `APP_ENV=production` is rejected at startup.

```bash
curl -X POST localhost:8080/v1/resources -H 'content-type: application/json' -d '{"title":"first"}'
curl 'localhost:8080/v1/resources?limit=10&offset=0'
curl localhost:8080/health/ready
```

## Layout

```
cmd/api/              Process entrypoint: config, wiring, signal handling
internal/config/      Typed configuration loaded and validated once at startup
internal/database/    Connection pool, health ping, lifecycle
internal/domain/      Entities and sentinel errors, free of transport concerns
internal/repository/  Persistence against GORM, context-aware
internal/service/     Business rules and authorization
internal/handler/     HTTP handlers, request and response shapes
internal/httpx/       Error envelope, validation, pagination, binding
internal/middleware/  Auth, claims access, request logging
internal/server/      Echo setup, middleware chain, routes, graceful shutdown
migrations/           SQL schema migrations
```

Dependencies point inward. Each layer depends on an interface it declares itself rather than on a concrete type from the layer below, which keeps packages independently testable: handlers are tested against a fake `ResourceService`, services against a stub repository.

## Endpoints

| Method | Path                  | Description                              |
| ------ | --------------------- | ---------------------------------------- |
| GET    | `/health/live`        | Liveness, no dependency checks           |
| GET    | `/health/ready`       | Readiness, verifies the database         |
| GET    | `/v1/resources`       | List, paginated via `limit` and `offset` |
| POST   | `/v1/resources`       | Create                                   |
| GET    | `/v1/resources/me`    | List resources owned by the caller       |
| GET    | `/v1/resources/:id`   | Fetch one                                |
| PATCH  | `/v1/resources/:id`   | Partial update, owner only               |
| DELETE | `/v1/resources/:id`   | Soft delete, owner only                  |

`Resource` is a placeholder entity. Copy `internal/domain/resource.go`, `internal/repository/resource.go`, `internal/service/resource.go`, and `internal/handler/resource.go` as a set when adding a real one, then register it in `internal/server/server.go`.

### Response shapes

Collections are wrapped with pagination metadata, single items with a `data` key, and every error uses one envelope:

```json
{ "data": [], "meta": { "total": 0, "limit": 20, "offset": 0 } }
```

```json
{
  "error": {
    "code": "validation_failed",
    "message": "request validation failed",
    "fields": { "title": "is required" },
    "request_id": "0Hq3..."
  }
}
```

Sentinel errors from `internal/domain` are translated to status codes centrally in `internal/httpx.ErrorHandler`, so handlers return errors rather than writing responses. Internal errors are logged in full and reported to clients as a generic message.

## Configuration

Every setting is read from the environment, validated at startup, and reported as a single aggregated error listing all problems at once. `.env` is loaded only if present, once, and never during request handling. Override the path with `ENV_FILE`. See `.env.example` for the full set.

| Variable                | Default       | Notes                                      |
| ----------------------- | ------------- | ------------------------------------------ |
| `APP_ENV`               | `development` | `development`, `staging`, or `production`  |
| `APP_PORT`              | `8080`        |                                            |
| `SERVER_REQUEST_TIMEOUT`| `10s`         | Per-request deadline                       |
| `SERVER_RATE_LIMIT`     | `20`          | Requests per second per client IP          |
| `SERVER_BODY_LIMIT`     | `1M`          | Maximum request body size                  |
| `CORS_ALLOW_ORIGINS`    | `*`           | Comma-separated; narrow this in production |
| `TRUSTED_PROXIES`       | empty         | Comma-separated CIDRs; see below           |
| `PG_*`                  | see example   | Host, credentials, pool, and SSL mode      |
| `AUTH_DISABLED`         | `false`       | Local bypass; forbidden in production      |
| `LOG_LEVEL`             | `info`        | `debug`, `info`, `warn`, `error`           |
| `LOG_FORMAT`            | `json`        | `json` or `text`                           |

### Client IP and trusted proxies

Rate limiting and request logs key off the client IP. By default the server reads it from the socket and **ignores** `X-Forwarded-For` and `X-Real-IP`, so a client cannot spoof its identity to bypass the rate limiter (CVE-2026-118653).

Behind a load balancer the socket address is the balancer, so set `TRUSTED_PROXIES` to the CIDRs you actually control:

```bash
TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12
```

Only `X-Forwarded-For` values arriving from those ranges are honored; loopback and private ranges are not trusted implicitly. Never set this to `0.0.0.0/0` — that restores the spoofable behavior.

## Development

```bash
make help     # list targets
make check    # tidy, vet, lint, test
make test     # go test -race -shuffle=on
make cover    # coverage summary
make lint     # golangci-lint
make build    # static binary in bin/
```

Migrations in `migrations/` are applied automatically by the Compose Postgres service on first boot. Schema is managed by SQL rather than GORM auto-migration so that production changes are reviewable and ordered.

## Deployment

`make docker-build` produces a static, non-root image on `distroless/static`.

Point your platform's health check at `/health/ready`, which verifies the database, and its liveness probe at `/health/live`, which does not. Supply `PG_PASSWORD`, `AUTH0_DOMAIN`, and `AUTH0_AUDIENCE` as secrets rather than environment literals, and set `TRUSTED_PROXIES` to the load balancer's CIDRs so rate limiting keys off the real client IP.

## License

MIT, see [LICENSE](LICENSE).
