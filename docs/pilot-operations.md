# Pilot operations

## Invitation and account setup

Issue an email-bound invitation (valid for seven days):

```sh
docker compose exec lumio /app/bin/lumio invite anna@example.com
```

Give the printed code to that photographer privately. They can choose **Have
an invitation? Create an account**, enter the same email, and set a password
of 12–128 bytes. Codes are consumed atomically on successful registration. The
server stores token hashes, never the raw invitation or session tokens.

Accounts use Argon2id (64 MiB, three iterations) and seven-day database
sessions. Cookies always use `__Host-lumio-session`, `Secure`, `HttpOnly`,
`Path=/`, and `SameSite=Strict`, with no Domain attribute. Local development
uses the browser's localhost secure-cookie exception; use the literal
`localhost`, not an IP address. Production requires HTTPS at the configured
dashboard origin. State-changing API requests require that exact `Origin`;
authenticated writes also require `X-CSRF-Token` from `GET /api/auth/me`. API
responses are not cached.

## Password recovery

The pilot uses operator-assisted password recovery:

```sh
docker compose exec lumio /app/bin/lumio reset-link anna@example.com
```

Verify the requester's ownership of that email and deliver the printed link
privately to that address. Links expire after 30 minutes, replace any previous
reset link, and can be used once. The token is in the URL fragment and is
removed from the address bar by the dashboard. Resetting a password revokes
all sessions. There is no public reset-request endpoint or email delivery
integration yet. Do not send invitation codes or reset links to shared logs or
channels.

## Current product limits

The dashboard reserves a subdomain and displays its private status. Uploads,
editing, and publication belong to later phases. Authentication concurrency is
bounded to protect Argon2 memory usage; full abuse rate limiting belongs to
Phase 6. Expired records cannot authenticate but are not yet periodically
pruned.
