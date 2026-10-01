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

Use k3s with the `local-path` StorageClass, Traefik v3 and its Middleware CRD.
PostgreSQL and Garage each have one
Recreate Deployment and a persistent claim. This is a single-node pilot setup,
with no replication or protection against losing that node. Keep the PVCs when
upgrading. Garage uses a single node with replication factor one and stores its
metadata and objects on its own PVC. Its S3 key is scoped to the `lumio` bucket.

Configure Traefik only in `../ansible-k3s`: the role there creates the DNS-01
provider credential Secret from controller environment variables, supports
staging/production ACME files, one replica, Recreate updates and protected
persistent certificate storage. Follow that repo's `docs/configuration.md`.
Create DNS-only A records for `lumio.nightnoryu.com` and
`*.lumio.nightnoryu.com` to the ingress IP. Add AAAA only for working IPv6. Issue
against staging first, then switch to production. The Ingress explicitly
requests one apex + wildcard certificate; new photographer subdomains need no
new resources. Traefik's
[Ingress TLS annotations](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/ingress/)
select the resolver and certificate domains.

### Beget DNS-01 challenge aliases

Beget's `dns/getData` can fail for DNS-only `_acme-challenge` names even when the
parent zone is managed by Beget. The Lego provider reads that name before
writing its TXT record, so certificate issuance stops with `METHOD_FAILED`.
Use dedicated Beget-managed subdomains as validation targets:

1. In Beget's **Domains and subdomains** panel, create managed subdomains
   `acme-lumio.nightnoryu.com` and `acme-grafana.nightnoryu.com`. These names
   must not serve traffic or hold other records; the Beget provider replaces
   their record sets while solving challenges.
2. In the `nightnoryu.com` DNS zone, add these explicit CNAME records:

   - `_acme-challenge.lumio.nightnoryu.com` → `acme-lumio.nightnoryu.com`
   - `_acme-challenge.grafana.nightnoryu.com` → `acme-grafana.nightnoryu.com`

   The first name handles both `lumio.nightnoryu.com` and its wildcard
   certificate. Lego follows each CNAME and asks Beget to write TXT records at
   its target.
3. Before restarting Traefik, verify that Beget's `dns/getData` API returns
   `success` for both target names and that public DNS resolves both CNAMEs.
   A DNS record alone is insufficient if `getData` still fails for its target.
   Retry staging issuance and inspect Traefik's ACME logs and served
   certificate. Switch to the production CA only after staging works.

If Beget cannot create managed validation targets or read them through
`dns/getData`, delegate these challenge CNAMEs to a zone managed by a DNS
provider with a working Traefik integration. Do not point them at the main
`nightnoryu.com` name: Beget's `changeRecords` replaces the record set there.

Check the trusted proxy CIDR in `k8s/prod/configmap.yaml` against the actual
cluster pod CIDR. The Lumio namespace has no NetworkPolicies, so other pods in
the cluster can reach its Services. Keep Traefik's forwarded-header trust
restricted to actual upstream proxies, if any.

## First deployment sequence (operator runbook)

Run this sequence only when deployment is authorized:

1. Configure DNS and Traefik as above. Ensure registry images are readable by
   the cluster; add an imagePullSecret to the pod specs for private images.
2. Render the production overlay. Create its Namespace, generated ConfigMap and
   Secret, PVCs, PostgreSQL/Garage Deployments and Services first.
   Wait for both data services to become ready. Do not start web/worker yet.
3. Configure the dedicated private bucket below.
4. Run the `Apply Kubernetes Manifests` workflow or apply the migration Job
   before the remaining web/worker and Ingress manifests. The Job runs embedded
   migrations under a PostgreSQL advisory lock; a failed migration stops the
   rollout. Application startup also applies pending migrations. A missing
   storage lifecycle rule prevents application startup.
5. Verify rollout, trusted HTTPS, storage CORS and the full upload/publish path.

The deploy workflow requires the `prod` environment with `KUBECONFIG` and
`SOPS_AGE_KEY` secrets. It applies the namespace, config, secret and data
services first, waits for PostgreSQL and Garage, recreates and waits for the
migration Job, then applies the remaining resources and waits for web/worker.
The release workflow calls it after publishing the GitHub Release. Provision
Garage bucket CORS and lifecycle rules before the first rollout.

Garage creates the `lumio` bucket and its owner key on first boot using
`LUMIO_S3_ACCESS_KEY` and `LUMIO_S3_SECRET_KEY` from the Secret. Replace the
encrypted placeholders with a Garage key ID (`GK` plus 32 hex characters) and
a 64-character hex secret before deploying.
Forward Garage's S3 API locally (no admin API is routed) with
`kubectl -n lumio port-forward service/garage 3900:3900`. In another terminal,
use the AWS CLI with credentials from your secret store:

```sh
export AWS_ACCESS_KEY_ID="$LUMIO_S3_ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$LUMIO_S3_SECRET_KEY"
export AWS_DEFAULT_REGION=garage
aws --endpoint-url http://127.0.0.1:3900 s3api put-bucket-cors \
  --bucket lumio --cors-configuration file://k8s/prod/storage-cors.json
aws --endpoint-url http://127.0.0.1:3900 s3api \
  put-bucket-lifecycle-configuration --bucket lumio \
  --lifecycle-configuration file://k8s/prod/storage-lifecycle.json
```

Use these initialization commands only for the dedicated Lumio bucket; the
lifecycle command replaces existing rules. Unset the AWS credential variables
afterward. `storage-init` is development-only and is not used in production.
Garage's bucket CORS allows exactly the dashboard
origin. Check a browser preflight for PUT with Content-Type and If-None-Match,
and GET/HEAD with exposed ETag. All objects remain private. S3 ingress routes
only `/lumio/` without rewriting host, path or query; no Garage admin API or
root bucket listing is exposed. The `s3` slug is reserved.

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

- Apex, dashboard and two photographer subdomains have trusted TLS. The apex
  redirects to the dashboard until a landing page is available. Unknown
  portfolios return 404.
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
aws --endpoint-url http://127.0.0.1:3900 s3 sync \
  s3://lumio /secure-backup/lumio-objects
```

Forward PostgreSQL port 5432 separately. `pg_dump` prompts for its password;
do not place it in shell history. Copy the dump and objects off the cluster,
encrypt them, retain dated copies, and also preserve configuration, secrets,
the lifecycle/CORS files and Traefik's private ACME files. A local PVC and
Garage staging expiry are not backups. Restart web/worker after both copies
complete. A fresh backup directory avoids retaining deleted objects by mistake.

To restore, keep web/worker stopped and provision empty PostgreSQL 16 and Garage
volumes in an isolated environment. Create the database/user with the same
names and re-create the private bucket, CORS, lifecycle and application key.
Restore with `pg_restore --host=127.0.0.1 --username=lumio --dbname=lumio
--no-owner --exit-on-error lumio.dump`, then restore objects:

```sh
aws --endpoint-url http://127.0.0.1:3900 s3 sync \
  /secure-backup/lumio-objects s3://lumio
```

Restore secrets and
start the image version corresponding to the backup before upgrading. Check
row counts, login, original object access, public images and a new upload.
Record that recovery test before relying on these backups. For rollback across
incompatible migrations, restore the matching database/object snapshot; do not
assume downgrading the image reverses schema changes.
