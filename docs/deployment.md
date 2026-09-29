# k3s deployment

These files are prepared for deployment; nothing has been applied or tested on
a cluster. Production uses `lumio.ru`, `app.lumio.ru`, and `s3.lumio.ru`.
The DNS provider, public ingress IP, release image tags and secrets must be
supplied by the operator. The base/production layout follows `anon3anon/k8s`.

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
coordinate the database role and MinIO application-user password changes first.
The deploy workflow selects the matching version tags for both images after a
release. For a manual deploy, supply an image tag or use the tags committed in
`k8s/prod/kustomization.yaml`. Release CI publishes both images. Local Compose
continues using the development Dockerfiles in `backend/`.
Tool versions and dependency locks are pinned; exact image bytes also require
pinning upstream image digests and an immutable Alpine package mirror.

## Cluster prerequisites

Use k3s with the `local-path` StorageClass, its network-policy controller
enabled, Traefik v3 and its Middleware CRD. PostgreSQL and MinIO each have one
Recreate Deployment and a persistent claim. This is a single-node pilot setup,
with no replication or protection against losing that node. Keep the PVCs when
upgrading. The MinIO community image matches local development and runs as root
to write its data volume. The application uses a separate restricted MinIO
account.

Configure Traefik only in `../ansible-k3s`: the role there supports DNS-01 with
an existing provider-credential Secret, staging/production ACME files, one
replica, Recreate updates and protected persistent certificate storage. Follow
that repo's `docs/configuration.md`. Create DNS-only A records for `lumio.ru`
and `*.lumio.ru` to the ingress IP. Add AAAA only for working IPv6. Issue
against staging first, then switch to production. The Ingress explicitly
requests one apex + wildcard certificate; new photographer subdomains need no
new resources. Traefik's
[Ingress TLS annotations](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress/)
select the resolver and certificate domains.

Check the pod CIDR in `config.env` and Traefik labels in the NetworkPolicies.
Only Traefik may reach the web service; only web/worker may reach PostgreSQL.
MinIO allows web/worker and Traefik. Monitoring in namespace `monitoring` may
reach port 9090. Change that namespace selector to match your installation.
Do not disable network policy while trusting the whole pod CIDR. Keep Traefik's
forwarded-header trust restricted to actual upstream proxies, if any.

## First deployment sequence (operator runbook)

Run this sequence only when deployment is authorized:

1. Configure DNS and Traefik as above. Ensure registry images are readable by
   the cluster; add an imagePullSecret to the pod specs for private images.
2. Render the production overlay. Create its Namespace, generated ConfigMap and
   Secret, PVCs, PostgreSQL/MinIO Deployments, Services and NetworkPolicies
   first.
   Wait for both data services to become ready. Do not start web/worker yet.
3. Provision the dedicated private bucket and application account below.
4. Run the `Apply Kubernetes Manifests` workflow or apply the migration Job
   before the remaining web/worker and Ingress manifests. The Job runs embedded
   migrations under a PostgreSQL advisory lock; a failed migration stops the
   rollout. Application startup also applies pending migrations. A missing
   storage lifecycle rule prevents application startup.
5. Verify rollout, trusted HTTPS, storage CORS and the full upload/publish path.

The deploy workflow requires the `prod` environment with `KUBECONFIG` and
`SOPS_AGE_KEY` secrets. It applies the namespace, config, secret and data
services first, waits for PostgreSQL and MinIO, recreates and waits for the
migration Job, then applies the remaining resources and waits for web/worker.
The release workflow calls it after publishing the GitHub Release. Provision
the MinIO bucket, user, policy and lifecycle rules before the first rollout.

For storage provisioning, forward MinIO's API locally (no public console or
admin API is routed). In a separate terminal, use
`kubectl -n lumio port-forward service/minio 9000:9000`. With the MinIO `mc`
client, set a local alias using the root credentials from your secret store:

```sh
mc alias set lumio-admin http://127.0.0.1:9000 \
  "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
mc mb --ignore-existing lumio-admin/lumio
mc anonymous set none lumio-admin/lumio
mc ilm import lumio-admin/lumio < k8s/prod/storage-lifecycle.json
mc admin user add lumio-admin "$LUMIO_S3_ACCESS_KEY" "$LUMIO_S3_SECRET_KEY"
mc admin policy create lumio-admin lumio-app k8s/prod/storage-policy.json
mc admin policy attach lumio-admin lumio-app --user "$LUMIO_S3_ACCESS_KEY"
```

Use these initialization commands only for the dedicated Lumio bucket; the
lifecycle import replaces existing rules. Protect and remove the local `mc`
alias credentials afterward. `storage-init` is development-only and is not
used in production. MinIO's CORS environment allows exactly the dashboard
origin. Check a browser preflight for PUT with Content-Type and If-None-Match,
and GET/HEAD with exposed ETag. All objects remain private. S3 ingress routes
only `/lumio/` without rewriting host, path or query; no MinIO console, root
bucket listing or admin API is exposed. The `s3` slug is reserved.

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

- Apex, dashboard and two photographer subdomains have trusted TLS. Unknown
  portfolios return 404; the apex currently also returns 404 because the
  application has no landing page. Its HTTPS route is prepared.
- Upload, preview, publish and unpublish work; anonymous original requests fail.
- Restart each application/data Deployment and Traefik, checking stored photos,
  database records and the same certificate survive.
- Unauthorized pods cannot reach database/web ports; each client's rate limit
  uses its own IP. Logs contain no signed URLs or authentication tokens.
- Metrics are scraped and a controlled failing job triggers the configured
  alert.

## Manual backup and restore

No automatic backup jobs are installed. Before a release or manual backup,
stop web and worker and wait for requests/jobs to finish. Keep them stopped
through the database dump and object copy so both snapshots represent the same
state. Using local port forwards and credentials from your secret store:

```sh
pg_dump --host=127.0.0.1 --username=lumio --dbname=lumio \
  --format=custom --file=lumio.dump
mc mirror lumio-admin/lumio /secure-backup/lumio-objects
```

Forward PostgreSQL port 5432 separately. `pg_dump` prompts for its password;
do not place it in shell history. Copy the dump and objects off the cluster,
encrypt them, retain dated copies, and also preserve configuration, secrets,
the lifecycle/policy files and Traefik's private ACME files. A local PVC and
MinIO staging expiry are not backups. Restart web/worker after both copies
complete. A fresh backup directory avoids retaining deleted objects by mistake.

To restore, keep web/worker stopped and provision empty PostgreSQL 16 and MinIO
volumes in an isolated environment. Create the database/user with the same
names and re-create the private bucket, policy, lifecycle and application user.
Restore with `pg_restore --host=127.0.0.1 --username=lumio --dbname=lumio
--no-owner --exit-on-error lumio.dump`, then
`mc mirror /secure-backup/lumio-objects lumio-admin/lumio`. Restore secrets and
start the image version corresponding to the backup before upgrading. Check
row counts, login, original object access, public images and a new upload.
Record that recovery test before relying on these backups. For rollback across
incompatible migrations, restore the matching database/object snapshot; do not
assume downgrading the image reverses schema changes.
