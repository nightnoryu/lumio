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
`password_reset`, `photo`, `site_draft`, `site_revision`, and go-kita's
`schema_migration`. Timestamped up/down migrations live in
`backend/data/migrations/`; only up migrations run
automatically. Add new migrations rather than editing applied ones.

## Portfolio drafts and rendering

The draft is a validated JSONB document with an optimistic version. Saving locks
its owning site, checks the version and verifies every referenced photograph is
ready and belongs to that site. Media remain normalized in `photo`; drafts only
contain IDs and presentation text. Photo deletion takes the same site lock and
rejects references from saved drafts or the active published revision.

Revision documents are separate snapshots; a database trigger rejects updates.
Publishing validates the saved draft version and its content, snapshots the
exact processed image keys/dimensions, and switches the site’s active revision
in one transaction under the site row lock. Repeating a publish of the same
active draft version is idempotent. Draft writes never update revisions.
Unpublishing clears the active pointer; historical documents remain immutable,
but only active revision references retain media.

The preview calls the portfolio application service directly and renders
reusable `templ` components. It requires the dashboard host and an authenticated
owner, uses short-lived signed optimized-image URLs, and sends no-store/noindex
headers. The renderer receives content and resolved image URLs independently of
HTTP and storage. Public pages use the same components, with canonical and Open
Graph metadata from the published snapshot.

Public routing accepts exactly one valid, nonreserved slug beneath the
configured base domain and the configured dashboard origin's port. Canonical
URLs use the configured scheme/domain/port, never forwarded headers or arbitrary
host input. Only the dashboard host serves private API and preview routes.

The S3 bucket remains private. Public image paths identify a revision,
photograph and variant index; the application rechecks the active revision and
streams only that snapshot's optimized object through Go. No client-provided
storage key is accepted. HTML and images use `no-store` so unpublishing denies
subsequent requests; already delivered bytes cannot be recalled. Cache policy
and responsive image source selection remain Phase 6 work.
