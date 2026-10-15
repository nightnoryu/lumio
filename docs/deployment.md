# k3s deployment

## Build and render without deploying

Build from the repository root. The multi-stage Dockerfile generates API code,
templates and the React bundle before compiling the embedded Go executable:

```sh
docker build --target web -t ghcr.io/nightnoryu/lumio:VERSION .
docker build --target worker -t ghcr.io/nightnoryu/lumio-worker:VERSION .
# Install sops, age and ksops; provide the matching age private key via
# SOPS_AGE_KEY_FILE.
# Replace placeholders with independent random credentials, then set image tags.
sops k8s/prod/secret.enc.yaml
kustomize build --enable-alpha-plugins --enable-exec k8s/prod \
  > /tmp/lumio-rendered.yaml
```

Rendered YAML contains plaintext-equivalent secrets. Production uses the same
KSOPS generator as `anon3anon`: `secret-generator.yaml` decrypts the committed
`secret.enc.yaml` into `lumio-secrets`. `.sops.yaml` uses the same public age
recipient as that repository and encrypts only `data`/`stringData`. The
encrypted file initially contains placeholders, which must be replaced before
deployment. Never commit a decrypted Secret or private key. If changing
recipients, update `.sops.yaml` and run `sops updatekeys
k8s/prod/secret.enc.yaml` while the existing private key is available.

The generated Secret has a stable name. After changing it, restart web, worker
and the affected data-service Deployments so their environment is refreshed.
Changing PostgreSQL's Secret does not rotate an existing database role password;
coordinate the database role and Garage key rotation first.
The deploy workflow selects the matching version tags for both images after a
release. For a manual deploy, supply an image tag or use the tags committed in
`k8s/prod/kustomization.yaml`. Release CI publishes both images. Local Compose
continues using the development Dockerfiles in `backend/`.
Tool versions and dependency locks are pinned; exact image bytes also require
pinning upstream image digests and an immutable Alpine package mirror.

## Cluster prerequisites

This runbook assumes the cluster is already configured with DNS records,
Traefik v3, ACME certificate issuance and its Middleware CRD. It also assumes
the `local-path` StorageClass is available.
PostgreSQL and Garage each have one
Recreate Deployment and a persistent claim. This is a single-node pilot setup,
with no replication or protection against losing that node. Keep the PVCs when
upgrading. Garage uses a single node with replication factor one and stores its
metadata and objects on its own PVC. Its S3 key is scoped to the `lumio` bucket.

The production overlay currently contains the application's hostnames. Before
deploying to a different domain, update the host rules and TLS domain
annotations in `k8s/prod/web-ingress.yaml` and `k8s/prod/s3-ingress.yaml`, plus
the base domain and public origins in `k8s/prod/configmap.yaml`. Ensure these
names match the DNS and certificate
coverage already configured on the cluster. Photographer subdomains are routed
by the wildcard host rule.

Check the trusted proxy CIDR in `k8s/prod/configmap.yaml` against the actual
cluster pod CIDR. The Lumio namespace has no NetworkPolicies, so other pods in
the cluster can reach its Services. Keep Traefik's forwarded-header trust
restricted to actual upstream proxies, if any.

## First deployment sequence (operator runbook)

Run this sequence only when deployment is authorized:

1. Ensure registry images are readable by the cluster; add an imagePullSecret
   to the pod specs for private images. Confirm the existing DNS and ACME setup
   covers the hostnames configured in the production overlay.
2. Render the production overlay. Create its Namespace, generated ConfigMap and
   Secret, PVCs, PostgreSQL/Garage Deployments and Services first.
   Wait for both data services to become ready. Do not start web/worker yet.
3. Run the `Apply Kubernetes Manifests` workflow or apply the migration Job,
   then the storage initialization Job, before the remaining web/worker and
   Ingress manifests. The migration Job runs embedded migrations under a
   PostgreSQL advisory lock. The storage Job creates the dedicated bucket and
   configures CORS and lifecycle rules. A failed Job stops the rollout.
   Application startup also applies pending migrations and requires the storage
   lifecycle rule.
4. Verify rollout, trusted HTTPS, storage CORS and the full upload/publish path.

The deploy workflow requires the `prod` environment with `KUBECONFIG` and
`SOPS_AGE_KEY` secrets. It applies the namespace, config, secret and data
services first, waits for PostgreSQL and Garage, recreates and waits for the
migration Job, then recreates and waits for the storage initialization Job.
Only then does it apply the remaining resources and wait for web/worker. The
release workflow calls it after publishing the GitHub Release.

Garage creates the `lumio` bucket and its owner key on first boot using
`LUMIO_S3_ACCESS_KEY` and `LUMIO_S3_SECRET_KEY` from the Secret. Replace the
encrypted placeholders with a Garage key ID (`GK` plus 32 hex characters) and
a 64-character hex secret before deploying. The storage Job runs the same
`storage-init` command used in development. It configures only the bucket named
by `LUMIO_S3_BUCKET`, using `LUMIO_DASHBOARD_ORIGIN` for CORS. On each deploy,
it replaces that bucket's CORS and lifecycle rules; keep any other policies in
the application initializer before deploying. Garage's bucket CORS allows
exactly the dashboard origin. Check a browser preflight for PUT with lowercase
`content-type` and `if-none-match` request headers, and GET/HEAD with exposed
ETag. All objects
remain private. S3 ingress routes only `/lumio/` without rewriting host, path
or query; no Garage admin API or root bucket listing is exposed. The `s3` slug
is reserved.

## Operations and verification

Web has database readiness and process liveness probes. Worker probes check its
private metrics listener, which starts after database/storage verification;
they do not detect a stalled job. Both emit JSON logs. Recreate web updates
avoid mixed frontend asset versions and cause brief downtime. Keep one replica
until shared asset retention is implemented. Web has one CPU and 1 GiB memory
to accommodate concurrent Argon2 password checks. Authentication work has no
global concurrency bound; monitor memory and tune limits if pilot traffic grows.
Worker has two CPUs, 1 GiB memory,
and bounded temporary disk; tune against real image workloads.

If Prometheus Operator is installed, use the separate `k8s/monitoring` overlay.
Configure Prometheus's PodMonitor/PrometheusRule namespace and label selectors
to discover these objects, and configure an Alertmanager receiver. This overlay
does not install a monitoring stack or send notifications by itself. Scrape
annotations are also present for an existing annotation-based collector; do
not scrape twice. Metrics and database/storage ports have no public Services.

Before inviting users, verify:

- The configured apex, dashboard and two photographer subdomains have trusted
  TLS. The apex redirects to the dashboard until a landing page is available.
  Unknown portfolios return 404.
- Upload, preview, publish and unpublish work; anonymous original requests fail.
- Restart each application/data Deployment and Traefik, checking stored photos,
  database records and the same certificate survive.
- Unauthorized pods cannot reach database/web ports; each client's rate limit
  uses its own IP. Logs contain no signed URLs or authentication tokens.
- Metrics are scraped and a controlled failing job triggers the configured
  alert.

## Manual backup and restore

No automatic backup jobs are installed. From the repository root, with a
production `kubectl` context and local `kubectl`, AWS CLI, `base64`, and
`sha256sum` available, run:

```sh
./ops/backup.sh --check
./ops/backup.sh
```

If `kubectl` is available only on an SSH host, set `KUBE_SSH_HOST` for both
commands, for example `KUBE_SSH_HOST=projects-prod-k3s ./ops/backup.sh --check`.
This mode uses a local AWS CLI and an SSH tunnel to Garage. The remote host
needs `kubectl`; local port 13900 and remote port 23900 must be free. The
`--check` command reads deployment, pod, service, and Secret metadata and
checks PostgreSQL tools without stopping the application or copying data.

The script expects one web and one worker replica. It scales both to zero,
waits for their pods to exit, streams a PostgreSQL custom-format dump, and
copies the private `lumio` bucket through a local Garage port forward. It reads
the Garage key from `lumio-secrets` without displaying it. The snapshot is
published as a dated `lumio-YYYYMMDDTHHMMSSZ` directory only after both copies
finish; the script attempts to restart web and worker even if backup fails. Use
`KUBE_NAMESPACE` to select another namespace or `LUMIO_BACKUP_S3_PORT` if local
port 13900 is occupied; `LUMIO_BACKUP_REMOTE_S3_PORT` changes the SSH host's
port 23900. Keep writes stopped from any other clients during the
snapshot. Expect a short application outage.

Copy the directory off the cluster, encrypt it, retain dated copies, and also
preserve configuration, secrets, the storage initialization code, and Traefik's
private ACME files. A local PVC and Garage staging expiry are not backups.

To restore a complete backup into the matching PostgreSQL 16 and Garage
installation, use the corresponding application image version and run:

```sh
./ops/restore.sh --yes lumio-YYYYMMDDTHHMMSSZ
```

The target must already have the `lumio` database, bucket, secrets and storage
configuration. The script verifies checksums, stops web and worker, drops and
recreates the database, then syncs objects with `--delete`. This
overwrites newer data in both stores. If restore fails, it leaves web and
worker stopped for inspection. Keep other clients from writing during recovery.
Check row counts, login, original object access, public images and a new upload.
Record that recovery test before relying on these backups. For rollback across
incompatible migrations, restore the matching database/object snapshot; do not
assume downgrading the image reverses schema changes.
