<h1 align="center">Lumio</h1>
<p align="center"><i>Your work, beautifully framed.</i></p>

<p align="center">
    <a href="https://github.com/nightnoryu/lumio/releases"><img src="https://img.shields.io/github/release/nightnoryu/lumio.svg?cache-control=no-cache"></a>
    <a href="https://github.com/nightnoryu/lumio/blob/main/LICENSE"><img src="https://img.shields.io/github/license/nightnoryu/lumio?cache-control=no-cache"></a>
    <a href="https://github.com/nightnoryu/lumio/actions/workflows/ci.yml"><img src="https://github.com/nightnoryu/lumio/actions/workflows/ci.yml/badge.svg?cache-control=no-cache"></a>
</p>

## Development

Install [mise](https://mise.jdx.dev) and Docker with Compose, then run:

```sh
mise install
mise run dev
```

Open **http://localhost:3000**. Traefik routes `/api/*`, `/healthz`, and
`/livez` to Go; Vite serves the dashboard and hot reloads frontend edits over
the same origin. Go changes need `mise run dev:reload`. Ports 3000, 5432,
9000, and 9001 must be free.

```sh
mise run dev:logs      # Follow container logs
mise run dev:ps        # Inspect service health
mise run dev:reload    # Rebuild Go and restart the backend
mise run dev:rebuild   # Rebuild container images after Dockerfile changes
mise run dev:down      # Stop services; retain database and object storage
mise run              # Build, test, type-check, lint, and check Go formatting
mise run build        # Build backend/bin/lumio with the React assets embedded
mise run test
mise run lint
mise run backend:generate  # Regenerate ogen code from api/publicapi.yml
```

`backend:build`, `test`, and `lint` build frontend assets as needed. Plain
`go build` or `go test` requires `mise run backend:generate` and
`mise run web:build` first. Generated API code, frontend output, and binaries are ignored and rebuilt
by mise. Frontend dependency
changes require updating the lockfile and refreshing the development volume:
`docker compose exec lumio-web pnpm install --frozen-lockfile`, then
`docker compose restart lumio-web`.

PostgreSQL is available at `localhost:5432` (database/user `lumio`, password
`lumio-local-only`). MinIO exposes S3 at `http://localhost:9000` and its console
at `http://localhost:9001` (user `lumio-local`, password `lumio-local-only`).
These credentials are for local development. Override them in an ignored
`.env` file using `LUMIO_DB_PASSWORD`, `LUMIO_MINIO_USER`, and
`LUMIO_MINIO_PASSWORD`. Volumes preserve data across restarts; changing the
PostgreSQL environment password does not change an existing database role.
Bucket creation and application uploads belong to phase 3. The local MinIO
image is the [alpine-docker community build](https://hub.docker.com/r/alpine/minio/)
of `RELEASE.2025-10-15T17-29-55Z`, pinned by release tag; official MinIO image
repositories were unavailable when this setup was verified. The local MinIO
container runs as root because that image leaves its `/data` volume owned
by root.

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

The current mise build targets Linux amd64. Open `http://localhost:8080`. `/api/status` returns `{"status":"ok"}`.
`/livez` checks process liveness; `/healthz` checks PostgreSQL readiness and
returns 503 if unavailable. Startup verifies database connectivity. SIGINT
and SIGTERM allow up to 10 seconds for active requests to finish.

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `LUMIO_SERVE_REST_ADDRESS` | `:8080` | HTTP listen address |
| `LUMIO_LOG_LEVEL` | `info` | Structured JSON log level |
| `LUMIO_DB_HOST` | required | PostgreSQL host |
| `LUMIO_DB_PORT` | `5432` | PostgreSQL port |
| `LUMIO_DB_NAME` | required | Database name |
| `LUMIO_DB_USER` | required | Database user |
| `LUMIO_DB_PASSWORD` | required | Database password |
| `LUMIO_DB_MAX_CONN` | `10` | Maximum open and idle connections |
| `LUMIO_DB_CONN_LIFETIME` | `60s` | Maximum connection lifetime; Go duration syntax |

Missing or invalid settings fail startup. MinIO settings configure the local
storage container only; phase 1 does not connect the application to S3.

## Architecture

The Go module lives in `backend/`; React source lives in `web/`.
`backend/cmd/lumio/` owns configuration, process lifecycle, and dependency
wiring. `backend/internal/transport/http/` handles HTTP and the generated API;
`backend/internal/webui/` embeds the Vite production build.

As business features are introduced, put pure entities and invariants in
`backend/internal/domain/`, use cases and external dependency interfaces in
`backend/internal/app/`, and concrete PostgreSQL/S3 adapters in
`backend/internal/infrastructure/`. Dependencies point inward: application
code imports domain code, adapters implement application interfaces, and HTTP
handlers call use cases. Domain code must not import HTTP, SQL, SDK, or
infrastructure packages. These directories will be created when they contain
real code. The current status endpoint is presentation-only.

The OpenAPI contract is `api/publicapi.yml`; generated files in
`backend/api/server/publicapi/` must not be edited manually. Phase 1 contains
no authentication, database schema, uploads, or portfolio pages.

## 📜 License

Distributed under the MIT License. See [License](/LICENSE) for more information.
