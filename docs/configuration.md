# Configuration

## Local services

PostgreSQL is available at `localhost:5432` (database/user `lumio`, password
`lumio-local-only`). Garage exposes S3 at `http://localhost:3900`. Compose
supplies local development S3 credentials. Override them in an ignored `.env`
with `LUMIO_S3_ACCESS_KEY` and `LUMIO_S3_SECRET_KEY` if needed. Garage key IDs
start with `GK` followed by 32 hex characters; secrets are 64 hex characters.
Set `LUMIO_DB_PASSWORD` there as needed. Volumes preserve data across restarts;
changing credentials after initializing Garage or PostgreSQL does not rotate
existing accounts. Garage
creates the private `lumio` bucket and key on first boot. The storage init step
sets dashboard CORS and one-day expiry for `uploads/` before the API starts.

## Standalone executable

After `mise run build`, the executable needs PostgreSQL, initialized S3
storage, and environment configuration, but no Node.js process or frontend
files:

```sh
LUMIO_DB_HOST=localhost \
LUMIO_DB_NAME=lumio \
LUMIO_DB_USER=lumio \
LUMIO_DB_PASSWORD=lumio-local-only \
LUMIO_S3_ENDPOINT=http://localhost:3900 \
LUMIO_S3_PUBLIC_ENDPOINT=http://localhost:3900 \
LUMIO_S3_REGION=garage \
LUMIO_S3_BUCKET=lumio \
LUMIO_S3_ACCESS_KEY=GK11111111111111111111111111111111 \
LUMIO_S3_SECRET_KEY=1111111111111111111111111111111111111111111111111111111111111111 \
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

<!-- markdownlint-disable MD013 -->

| Variable                   | Default                 | Meaning                                               |
|----------------------------|-------------------------|-------------------------------------------------------|
| `LUMIO_DASHBOARD_ORIGIN`   | `http://localhost:3000` | Dashboard origin; HTTPS except localhost              |
| `LUMIO_BASE_DOMAIN`        | `localhost`             | Portfolio domain; production uses `app.<base domain>` |
| `LUMIO_SERVE_REST_ADDRESS` | `:8080`                 | HTTP listen address                                   |
| `LUMIO_LOG_LEVEL`          | `info`                  | Structured JSON log level                             |
| `LUMIO_DB_HOST`            | required                | PostgreSQL host                                       |
| `LUMIO_DB_PORT`            | `5432`                  | PostgreSQL port                                       |
| `LUMIO_DB_NAME`            | required                | Database name                                         |
| `LUMIO_DB_USER`            | required                | Database user                                         |
| `LUMIO_DB_PASSWORD`        | required                | Database password                                     |
| `LUMIO_DB_MAX_CONN`        | `10`                    | Maximum open and idle connections                     |
| `LUMIO_DB_CONN_LIFETIME`   | `60s`                   | Maximum connection lifetime (Go duration)             |

<!-- markdownlint-enable MD013 -->

Missing or invalid settings fail startup. Compose supplies all six required S3
settings to the backend.

## Media storage and worker

<!-- markdownlint-disable MD013 -->

| Variable                    | Default                 | Meaning                                                |
|-----------------------------|-------------------------|--------------------------------------------------------|
| `LUMIO_S3_ENDPOINT`         | required                | Server/worker S3 endpoint; path-style                  |
| `LUMIO_S3_PUBLIC_ENDPOINT`  | required                | Browser S3 endpoint; use HTTPS with an HTTPS dashboard |
| `LUMIO_S3_REGION`           | required                | Signing region                                         |
| `LUMIO_S3_BUCKET`           | required                | Private media bucket                                   |
| `LUMIO_S3_ACCESS_KEY`       | required                | Garage key ID                                          |
| `LUMIO_S3_SECRET_KEY`       | required                | Garage secret                                          |
| `LUMIO_MEDIA_FILE_BYTES`    | `52428800`              | Max original size (50 MiB; up to 500 MiB)              |
| `LUMIO_MEDIA_STORAGE_BYTES` | `2147483648`            | Original storage per site (2 GiB)                      |
| `LUMIO_MEDIA_PHOTOS`        | `100`                   | Photo reservations per site                            |

<!-- markdownlint-enable MD013 -->

Run `lumio worker` as a separate process with the same database/storage
settings. The worker image supplies libvips 8.17, fonts, a 1 GiB Compose memory
limit and two CPUs. The web process needs no libvips. Keep clocks synchronized
because job deadlines use PostgreSQL claim timestamps.

For standalone local setup, run `lumio storage-init` once with the same
settings. This explicit **development-only** command creates the bucket, sets
dashboard CORS and replaces lifecycle rules with one-day expiry for
`uploads/`. Never use it against a bucket with other lifecycle policies.
Production provisioning must configure the private bucket, block anonymous
access, and add that staging rule itself. API and worker startup verify the
lifecycle rule exists. Runtime credentials need Garage read/write bucket access;
initialization also needs owner access. Do not grant browser credentials.

Configure bucket CORS for the exact dashboard origin, methods PUT/GET/HEAD,
request headers `Content-Type` and `If-None-Match`, and expose `ETag`. Compose
sets Garage bucket CORS from `LUMIO_DASHBOARD_ORIGIN`; change it when using a
standalone dashboard port. Do not rewrite the host/path of signed URLs. All S3
objects stay private. Owners receive five-minute preview URLs for optimized
variants only. Public portfolios stream optimized images through Go after
checking the active published revision. Do not enable anonymous bucket access.

## Public portfolio addresses

A portfolio URL uses `<slug>.<LUMIO_BASE_DOMAIN>` and the scheme and explicit
port of `LUMIO_DASHBOARD_ORIGIN`. For production, configure an HTTPS dashboard
origin such as `https://app.example.com` and the base domain `example.com`, then
route wildcard DNS/ingress to the Go service. Forward the original Host header;
forwarded host/protocol headers are not used for tenant resolution or canonical
URLs. The local defaults produce `http://anna.localhost:3000/` for the slug
`anna`. Unknown, unpublished, nested, reserved or malformed portfolio hostnames
return 404.

### HTTP caching, security and observability

The production executable embeds the dashboard, recovery script and portfolio
viewer. Only `/` is a dashboard page today; unknown paths, missing assets and
unknown API routes return 404. Add explicit dashboard routes if client routing
is introduced. Hashed JS/CSS assets use one year of immutable caching. HTML, the
recovery script and the portfolio viewer revalidate. API responses, previews and
public HTML use `no-store`. Published image URLs include the immutable revision
and variant index; they use `private, no-cache` and ETags. Each reuse checks the
active publication before returning 304, so unpublishing also revokes cached
URLs. Already downloaded photographs cannot be recalled. Revalidation currently
opens the S3 object but avoids retransmitting its body.

Old dashboard assets are not retained. A stable `/recovery.js`, loaded before
bundles, catches missing JS/CSS and dynamic imports and offers a reload. It
never reloads automatically, preserving an open editor until the user chooses to
reload. The reload revalidates HTML and loads current assets without clearing
the session. This strategy applies to HTML delivered from this release onward.
Deploy a single web replica or switch traffic atomically; mixed old/new replicas
need shared asset retention. Verify recovery when changing the frontend build or
loader.

Security headers apply to every application response. Scripts are restricted to
the same origin; the dashboard permits uploads and previews only at the
configured `LUMIO_S3_PUBLIC_ENDPOINT`. Inline styles remain permitted for templ
styles and Uppy. HTTPS origins enable HSTS. Cookies remain host-only, Secure,
HttpOnly and SameSite=Strict; public sites never set dashboard cookies.

Authentication attempts share a 10/minute limit per client IP. Authenticated API
access is limited to 600/minute per IP, writes to 120/minute per account, and
photo writes to 60/minute per account. Limits are per web process, reset on
restart, and return 429 with `Retry-After: 60`. Direct presigned S3 PUTs are
bounded by upload reservations, expiry, size validation and quotas rather than
the Go request limiter. `LUMIO_TRUSTED_PROXY_CIDRS` is a comma-separated list of
proxy CIDRs (empty by default). Only socket peers in these ranges may supply
`X-Forwarded-For`; chains are read from the right, stopping at the first
untrusted address. Configure the actual ingress CIDR and restrict direct access
to the Go port. Never trust all addresses. Without trusted proxies, clients
behind ingress share its IP limit.

`LUMIO_METRICS_ADDRESS` defaults to `127.0.0.1:9090`; an empty value disables
it. The web process and worker each serve a private Prometheus registry at
`/metrics` on this separate listener. Set distinct ports when running both on
one host. Compose uses `0.0.0.0:9090` within each container without publishing
it or routing it through Traefik. Scrape both processes over the private
network; do not expose this listener through public ingress. Runtime/process
metrics accompany `lumio_http_requests_total`,
`lumio_http_request_duration_seconds`, `lumio_media_operations_total` and
`lumio_media_operation_duration_seconds`. Media operations are `upload_create`,
`upload_complete`, `process`, and `cleanup`; outcomes are `success`, `error`,
and `exhausted` for abandoned processing attempts. Job metrics count attempts,
not unique photographs. `upload_complete` measures backend
verification/queueing, not browser-to-S3 transfer time.

Logging follows anon3anon: go-kita JSON logs with `LUMIO_LOG_LEVEL`, structured
route/status/duration fields for requests and operation/outcome/duration for
media. Request summaries omit URLs, query strings, cookies, bodies and client
identifiers; metric labels use bounded route/operation names. Health requests
are not logged.

Traefik is the sole compression layer. Compose attaches `lumio-compress` to the
Go API and public-site routers, excluding JPEG/PNG/WebP. The application does
not compress responses. Attach the equivalent middleware to all production Go
routers (including the embedded dashboard) when implementing Phase 7 ingress.
Local Vite serves the development dashboard, so production asset/header checks
must target the Go server, not Vite.
