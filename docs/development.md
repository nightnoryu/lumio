# Development

## Prerequisites

Install [mise](https://mise.jdx.dev) and Docker with Compose. Then run:

```sh
mise install
mise run dev
```

Open **[http://localhost:3000](http://localhost:3000)**. Traefik routes
`/api/*`, `/healthz`, `/livez`, `/preview/*` and `/portfolio.js` to Go; Vite
serves the dashboard and hot reloads frontend edits over the same origin. Go
changes need `mise run dev:reload`. Ports 3000, 5432 and 3900 must be free.

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

## Photography uploads

The dashboard accepts JPEG, PNG and WebP, shows upload progress and offers
cancellation/retry. Optional watermark text is baked into every variant. If a
transfer succeeded but its completion request failed, use **Check completed
upload** in the photo list. An expired upload must be removed and selected
again. Single PUT uploads restart from the beginning; multipart/resume is
deferred until real portfolio testing shows it is needed.

Upload creation reserves count and original bytes under a site row lock. Removal
hides a photo immediately, but quota is released only after object cleanup,
following signed URL/worker lease expiry (normally up to 16 minutes). Failed
photos retain their quota until removed. Storage also contains generated
variants and temporarily duplicated staged originals; these are outside the
original-byte quota. Completion verifies the S3 size/type, copies the object
into private durable storage, and atomically transitions the photo into the
PostgreSQL job queue. The worker verifies signatures/dimensions and fully
decodes the image before it becomes ready. A queued upload survives staging
expiry and worker downtime.

Jobs use `FOR UPDATE SKIP LOCKED`, ten-minute leases and a three-minute attempt
budget. Failed attempts retry up to three times. Crashed jobs are reclaimed;
lease tokens fence stale commits. Versioned variants are isolated by attempt,
with obsolete attempts pruned after success. Deletion and abandoned-upload
cleanup run in the worker; staging lifecycle also catches uploads finishing
after cleanup.

```sh
mise run backend:test:media  # Real libvips processing inside the worker image
# Local Garage integration uses the default development credentials:
docker compose run --rm lumio-storage-init
LUMIO_TEST_STORAGE=1 mise run backend:test:e2e
```

Automated image fixtures cover portrait/landscape, EXIF orientation, metadata
stripping, original preservation and invalid signatures. Real photographer
colour/sharpness review and interrupted large browser uploads remain pilot
checks.

## Portfolio editor

Edit profile details, select and order processed photographs, add categories,
services/prices and contact links, then choose Gallery or Editorial and
appearance options. **Save draft** persists incomplete portfolios;
**Preview saved portfolio** opens the authenticated, server-rendered page.
Save before previewing new edits. Use **Refresh uploaded photographs** after
processing completes.

Concurrent saves return a conflict instead of overwriting another window's
edits. Reloading a saved draft asks before discarding local changes. Remove
profile, cover and gallery references and save before deleting an uploaded
photograph. Photographs referenced by the live revision are also retained until
you publish a revision that no longer uses them, or unpublish.

Templates live in `backend/internal/portfolio/page.templ`. Run
`mise run backend:templates` after editing them; the backend build also
generates them. The pinned templ runtime and generator use the same version.
Generated `page_templ.go` is committed so ordinary Go checks can compile the
renderer.

Editor tests cover form entry and save failures; PostgreSQL tests cover
ownership, CSRF, stale saves, revision isolation, media retention and private
preview access. Template tests check appearance selection, escaping and hidden
prices. Manual mobile/desktop visual review with representative photographs
remains a pilot check.

## Publishing

Add your display name, biography, contact link, and descriptions for every
selected photograph. Save and preview the draft, then choose **Publish
portfolio**. **Publish saved changes** replaces the live snapshot; ordinary
saves remain private. **Unpublish portfolio** makes both the page and subsequent
public image requests return 404. The dashboard displays the live URL and
whether saved/unsaved edits differ from the published version. Optional page
title/description fields control search and sharing metadata; the cover or first
gallery image supplies the share image.

Compose routes `http://<slug>.localhost:3000/` to Go. Browsers normally resolve
`*.localhost` to loopback. If your resolver does not, add the chosen subdomain
to `/etc/hosts` or use: `curl --resolve anna.localhost:3000:127.0.0.1
http://anna.localhost:3000/`. After route configuration changes, recreate the
service with `docker compose up -d lumio` after building the executable.

Visitors can open a photograph in the full-screen viewer, use Previous/Next or
arrow keys, and close with Escape. Keyboard focus returns to the opened
photograph. Without JavaScript, gallery links open the optimized photograph
directly.

Publication integration tests exercise actual PostgreSQL revisions and HTTP host
routing with an object-store fixture. Browser checks at 1440×1000 and 390×844
cover both templates, image loading, width/overflow and viewer keyboard/focus
behavior with synthetic images. Real photograph review and deployed wildcard
HTTPS remain pilot/deployment checks.
