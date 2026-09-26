# Development

## Prerequisites

Install [mise](https://mise.jdx.dev) and Docker with Compose. Then run:

```sh
mise install
mise run dev
```

Open **http://localhost:3000**. Traefik routes `/api/*`, `/healthz`, and
`/livez` to Go; Vite serves the dashboard and hot reloads frontend edits over
the same origin. Go changes need `mise run dev:reload`. Ports 3000, 5432,
9000, and 9001 must be free.

## Common commands

```sh
mise run dev:logs      # Follow container logs
mise run dev:ps        # Inspect service health
mise run dev:reload    # Rebuild Go and restart the backend
mise run dev:rebuild   # Rebuild container images after Dockerfile changes
mise run dev:down      # Stop services; retain database and object storage
mise run              # Build, test, type-check, lint, and check Go formatting
mise run build         # Build backend/bin/lumio with React assets embedded
mise run test
mise run lint
mise run backend:generate  # Regenerate ogen code from api/publicapi.yml
```

`backend:build`, `test`, and `lint` build frontend assets as needed. Plain
`go build` or `go test` requires `mise run backend:generate` and
`mise run web:build` first. Generated API code, frontend output, and binaries
are ignored and rebuilt by mise. Frontend dependency changes require updating
the lockfile and refreshing the development volume:
`docker compose exec lumio-web pnpm install --frozen-lockfile`, then
`docker compose restart lumio-web`.

## Tests

Unit tests run with `mise run test` (or `mise run backend:test:unit` for Go
only) and do not require Docker. E2E tests are separate, under
`backend/test/e2e/`, with the `e2e` build tag:

```sh
mise run backend:test:e2e
```

E2E tests require a running Docker daemon. Testcontainers starts a disposable
PostgreSQL container on a dynamic port and removes it after the test. Requests
run through an HTTPS test server. No development database or credentials are
needed. CI runs E2E separately after the regular build and checks. Coverage
includes invitation redemption, ownership, CSRF/Origin validation, sessions,
logout, and password recovery. React tests exercise sign-in, site creation,
and sign-out.
