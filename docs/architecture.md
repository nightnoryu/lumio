# Architecture

## Repository layout

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
infrastructure packages. Repositories use go-kita over pgx with parameterized
SQL, following Cadence. Invitation redemption and password resets use explicit
transactions. Every private site query includes the authenticated owner;
multiple sites per account are supported in storage and API, while the initial
UI offers one.

## API and frontend

The OpenAPI contract is `api/publicapi.yml`; generated files in
`backend/api/server/publicapi/` must not be edited manually. The TypeScript
schema is generated with `openapi-typescript` and consumed through
`openapi-fetch`; `pnpm run generate` in `web/` regenerates it.

## Database

The schema uses singular table names: `user`, `session`, `site`, `invitation`,
`password_reset`, and go-kita's `schema_migration`. Timestamped up/down
migrations live in `backend/data/migrations/`; only up migrations run
automatically. Add new migrations rather than editing applied ones.
