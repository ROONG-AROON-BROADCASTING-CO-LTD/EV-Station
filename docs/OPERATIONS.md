# RBC operations

Scheduled jobs launch through `wscript.exe` and `Run-RBC-Hidden.vbs` so PowerShell starts hidden before it can flash a console. The wrapper waits for completion and propagates failures to Task Scheduler. Logs remain under `.runtime/operations`.

Run `scripts/Install-RBC-Operations.ps1` to register Windows tasks for the current operator: monitoring every five minutes and at sign-in, backup daily at 02:00, isolated restore testing Sunday at 03:00. Tasks require the operator to be signed in. Docker Desktop is a workstation service: this does not guarantee service before Windows sign-in or while this machine is powered off. An always-on server and an independent external monitor remain needed for unattended availability.

`Invoke-RBC-Operations.ps1 -Action Recover` starts existing deployed containers in dependency order, without building or migrating. Monitor checks API and public HTTPS and attempts recovery. Logs and status are in `.runtime/operations`. PostgreSQL backups are restricted to the operator and SYSTEM under `backups/automated`. Restore tests use a disposable container with no network or published ports, never the live database.

Local retention keeps the latest fourteen successful dumps (about 6.4 GB at the current 453 MB per dump). Optional private `.runtime/operations/config.json` fields:

```json
{"OffHostDirectory":"\\\\approved-server\\encrypted-backups", "AlertWebhook":"https://approved-monitor.example/webhook"}
```

Configure only approved storage and alert destinations. Without these fields, local backup and repair operate but **off-host backup and remote alerts are not configured**. Network storage should enforce encryption, restricted operator access, and retention. Copy checksums are verified. A local monitor cannot alert while its host is off. Monitor failures remain in the task result and local log; external monitoring is a separate requirement.

Authentication endpoints allow ten attempts per normalized email, route and source per fifteen minutes, plus 120 requests per source per minute. Successful logins clear the account-attempt counter. Oversized bodies are rejected. Counters are bounded, in memory, and reset on API restart; use shared counters before scaling. nginx overwrites `X-RBC-Client-IP` using the existing local Cloudflare tunnel's `CF-Connecting-IP`; API accepts it only from `AUTH_TRUSTED_PROXY_CIDRS`. Pin that setting to the actual nginx network in deployment. Keep the web/API published ports loopback-only: exposing nginx directly would invalidate this trusted-tunnel assumption. Users sharing a source can still share its request limit.

LINE notification recipients must be explicitly set in `LINE_NOTIFICATION_RECIPIENT_ID`. Signed webhooks no longer discover or change recipients, and legacy recipient rows are ignored. If this setting is empty, internal customer notifications remain disabled rather than using an unapproved destination.

Password changes invalidate tokens via a credential binding. On the first rollout of this change, old tokens without that binding require sign-in again. No database migration is needed. For a new database, public registration creates customers by default; set `BOOTSTRAP_OWNER_EMAIL` to the operator-approved email before its first registration. That address must pass OTP and an owner must not already exist. Existing owners remain unchanged; remove the bootstrap setting after provisioning.

Deploy only after tests: tag the running images for rollback; `docker compose build api web`; `docker compose up -d --no-deps --no-build api web`; verify public homepage, API readiness, protected routes, and authentication rate limiting. Roll back images, not the database or signing secrets. Never factory-reset Docker to fix availability.

Passwords are not stored in Git. Rotated operator credentials are delivered locally in `.runtime/credentials`; transfer them to a password manager and restrict that directory. No test credentials should appear in public documentation.
