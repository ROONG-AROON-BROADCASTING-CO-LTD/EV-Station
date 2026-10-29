# RBC operations

Run `scripts/Install-RBC-Operations.ps1` to register Windows tasks for the current operator: monitoring every five minutes and at sign-in, backup daily at 02:00, isolated restore testing Sunday at 03:00. Tasks require the operator to be signed in. Docker Desktop is a workstation service: this does not guarantee service before Windows sign-in or while this machine is powered off. An always-on server and an independent external monitor remain needed for unattended availability.

`Invoke-RBC-Operations.ps1 -Action Recover` starts existing deployed containers in dependency order, without building or migrating. Monitor checks API and public HTTPS and attempts recovery. Logs and status are in `.runtime/operations`. PostgreSQL backups are restricted to the operator and SYSTEM under `backups/automated`. Restore tests use a disposable container with no network or published ports, never the live database.

Local retention keeps the latest fourteen successful dumps (about 6.4 GB at the current 453 MB per dump). Optional private `.runtime/operations/config.json` fields:

```json
{"OffHostDirectory":"\\\\approved-server\\encrypted-backups", "AlertWebhook":"https://approved-monitor.example/webhook"}
```

Configure only approved storage and alert destinations. Without these fields, local backup and repair operate but **off-host backup and remote alerts are not configured**. Network storage should enforce encryption, restricted operator access, and retention. Copy checksums are verified. A local monitor cannot alert while its host is off. Monitor failures remain in the task result and local log; external monitoring is a separate requirement.

Authentication endpoints allow ten requests per normalized email per route per fifteen minutes, plus a process-wide ceiling of 120 authentication requests per minute. Oversized bodies are rejected. Counters are bounded, in memory, and reset on API restart. This protects the current single API instance; use shared counters or ingress protections before scaling to multiple instances. Account limits can temporarily block a targeted account, so external abuse monitoring remains advisable.

Deploy only after tests: tag the running images for rollback; `docker compose build api web`; `docker compose up -d --no-deps --no-build api web`; verify public homepage, API readiness, protected routes, and authentication rate limiting. Roll back images, not the database or signing secrets. Never factory-reset Docker to fix availability.

Passwords are not stored in Git. Rotated operator credentials are delivered locally in `.runtime/credentials`; transfer them to a password manager and restrict that directory. No test credentials should appear in public documentation.
