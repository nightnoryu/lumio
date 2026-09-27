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
root. Compose creates a private `lumio` bucket and configures one-day expiry for
`uploads/` staging objects before starting the API and worker.

## Standalone executable

After `mise run build`, the executable needs PostgreSQL, initialized S3 storage, and environment
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

## Media storage and worker

| Variable | Default | Meaning |
| --- | --- | --- |
| `LUMIO_S3_ENDPOINT` | `http://localhost:9000` | Server/worker S3 endpoint, path-style addressing |
| `LUMIO_S3_PUBLIC_ENDPOINT` | `http://localhost:9000` | Browser-reachable S3 endpoint for signatures; HTTPS with an HTTPS dashboard |
| `LUMIO_S3_REGION` | `us-east-1` | Signing region |
| `LUMIO_S3_BUCKET` | `lumio` | Private media bucket |
| `LUMIO_S3_ACCESS_KEY` | `lumio-local` | S3 key; replace outside development |
| `LUMIO_S3_SECRET_KEY` | `lumio-local-only` | S3 secret; replace outside development |
| `LUMIO_MEDIA_FILE_BYTES` | `52428800` | Maximum original size (50 MiB; configurable up to 500 MiB) |
| `LUMIO_MEDIA_STORAGE_BYTES` | `2147483648` | Original storage reservation per site (2 GiB) |
| `LUMIO_MEDIA_PHOTOS` | `100` | Photo reservations per site |

Run `lumio worker` as a separate process with the same database/storage settings.
The worker image supplies libvips 8.17, fonts, a 1 GiB Compose memory limit and
two CPUs. The web process needs no libvips. Keep clocks synchronized because
job deadlines use PostgreSQL claim timestamps.

For standalone local setup, run `lumio storage-init` once with the same settings.
This explicit **development-only** command creates the bucket, removes its public
policy and replaces lifecycle rules with one-day expiry for `uploads/`. Never
use it against a bucket with other lifecycle policies. Production provisioning
must configure the private bucket, block anonymous access, and add that staging
rule itself. API and worker startup verify the lifecycle rule exists. Runtime
credentials need object Get/Put/Copy/Delete, ListBucket and GetLifecycleConfiguration;
initialization additionally needs bucket creation, DeleteBucketPolicy and
PutLifecycleConfiguration. Do not grant browser credentials.

Configure bucket CORS for the exact dashboard origin, methods PUT/GET/HEAD,
request headers `Content-Type` and `If-None-Match`, and expose `ETag`.
Compose sets MinIO's allowed origin to `http://localhost:3000`; change it when
using a standalone dashboard port. Do not rewrite the host/path of signed URLs.
All objects stay private in Phase 3. Owners receive five-minute preview URLs
for optimized variants only. Publishing will grant access in Phase 5.
