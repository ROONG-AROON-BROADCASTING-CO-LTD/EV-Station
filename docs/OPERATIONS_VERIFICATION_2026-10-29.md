# Verification — 29 October 2026

- Public homepage restored and returns HTTP 200. API health returns `status: ok`.
- Deployed API and web images include authentication throttling and runtime Docker DNS resolution. API environment is production with authentication required.
- Controlled outage: stopped API and restarted nginx; frontend still returned 200. Running Monitor recovered API and public service to 200.
- Windows Task Scheduler Monitor completed with result 0. Tasks run with the signed-in operator's limited privileges, with process-scoped PowerShell execution policy matching the project launcher. No machine-wide execution policy was changed. No Windows reboot or pre-login availability was tested.
- All three existing local test/operator accounts received new random passwords; API login returned 200 for each. JWT signing secret rotated to invalidate existing sessions. Private delivery file: `.runtime/credentials/rotated-20261029.json` (excluded from Git).
- Live public login test on a nonexistent reserved test address: ten 401 responses followed by 429. Actual account login remains available.
- Local PostgreSQL custom-format backup SHA-256 verified; isolated full restore succeeded: 8 users, 22 sites, 130 reports, 1,193 metrics. Restore container had no network or published port and was removed afterward.
- Frontend 55 tests and lint passed. Go tests across all packages and go vet passed. API and web Docker builds passed. Existing frontend bundle-size warning remains.
- Existing images retained as `ev-station-api:rollback-20261029-operations` and `ev-station-web:rollback-20261029-operations`.

## Incomplete dependencies

No approved external storage or alert recipient has been supplied. Therefore off-host backups and external outage alerts remain **not configured**, not passed. Scheduled backup runs at 02:00 and restore testing Sunday 03:00 while the operator is signed in. A workstation being powered off, logged out, disconnected or unhealthy cannot be repaired or monitored externally by these local tasks. Moving service to an always-on host and configuring an independent monitor remains necessary for unattended production operation.
