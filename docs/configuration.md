# Configuration

## Local services

PostgreSQL is available at `localhost:5432` (database/user `lumio`, password
`lumio-local-only`). MinIO exposes S3 at `http://localhost:9000` and its
console at `http://localhost:9001` (user `lumio-local`, password
`lumio-local-only`). These credentials are for local development. Override
them in an ignored `.env` file using `LUMIO_DB_PASSWORD`,
`LUMIO_MINIO_USER`, and `LUMIO_MINIO_PASSWORD`. Volumes preserve data across
restarts; changing the PostgreSQL environment password does not change an
existing database role.

The local MinIO image is the [alpine-docker community build](https://hub.docker.com/r/alpine/minio/)
of `RELEASE.2025-10-15T17-29-55Z`, pinned by release tag; official MinIO image
repositories were unavailable when this setup was verified. The local MinIO
container runs as root because that image leaves its `/data` volume owned by
root. MinIO settings configure the local storage container only; the current
application phase does not connect the application to S3.

## Standalone executable

After `mise run build`, the executable needs PostgreSQL and environment
configuration, but no Node.js process or frontend files:

```sh
LUMIO_DB_HOST=localhost \
LUMIO_DB_NAME=lumio \
LUMIO_DB_USER=lumio \
LUMIO_DB_PASSWORD=lumio-local-only \
./backend/bin/lumio
```

The current mise build targets Linux amd64. Set
`LUMIO_DASHBOARD_ORIGIN=http://localhost:8080` to open the standalone
dashboard at `http://localhost:8080`. `/api/status` returns
`{"status":"ok"}`. `/livez` checks process liveness; `/healthz` checks
PostgreSQL readiness and returns 503 if unavailable. Startup verifies database
connectivity and applies embedded PostgreSQL migrations under an advisory lock.
SIGINT and SIGTERM allow up to 10 seconds for active requests to finish.

## Environment variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `LUMIO_DASHBOARD_ORIGIN` | `http://localhost:3000` | Exact dashboard origin; HTTPS required except on localhost |
| `LUMIO_BASE_DOMAIN` | `localhost` | Portfolio base domain; production dashboard must use `app.<base domain>` |
| `LUMIO_SERVE_REST_ADDRESS` | `:8080` | HTTP listen address |
| `LUMIO_LOG_LEVEL` | `info` | Structured JSON log level |
| `LUMIO_DB_HOST` | required | PostgreSQL host |
| `LUMIO_DB_PORT` | `5432` | PostgreSQL port |
| `LUMIO_DB_NAME` | required | Database name |
| `LUMIO_DB_USER` | required | Database user |
| `LUMIO_DB_PASSWORD` | required | Database password |
| `LUMIO_DB_MAX_CONN` | `10` | Maximum open and idle connections |
| `LUMIO_DB_CONN_LIFETIME` | `60s` | Maximum connection lifetime; Go duration syntax |

Missing or invalid settings fail startup. Configure local-only MinIO credentials
through the `.env` variables described above.
