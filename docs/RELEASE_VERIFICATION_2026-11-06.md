# Release verification — 6 November 2026

## Deployed changes

- `d90ed4b`: analysis evidence handling, Gemini recovery, OTP resend, report refresh, PDF downloads, notification pairing and road diagnostics. Also fixes a blank analysis page when pending metrics are null.
- `6355a61`: reproducible web Docker build with pinned pnpm and synchronized standalone lockfile.
- `ca6f2c8`: customer submission channel chooser. Desktop offers website submission or OA chat; mobile additionally offers the configured LIFF form and tracking. This does not configure a staff notification recipient.
- Web deployed at approximately 10:58 Asia/Bangkok. API and migration 27 were deployed earlier in this session. Road updater rebuilt and restarted; health check passed.

## Executed verification

- Backend: `go test ./... -count=1 && go vet ./...` inside the existing Go build image: passed.
- Frontend: `npm --prefix apps/web test`: 153 tests across 21 files passed.
- `npm --prefix apps/web run lint`, `build`, `test:seo`: passed. Build still warns about large chunks and upstream Zod comment annotations.
- After the final button contrast adjustment, the three affected test files were rerun: 10 tests passed. Production Docker web build passed.
- `scripts/test-e2e.ps1`: 16 isolated desktop/mobile tests passed after fixing the null-metrics bug and updating PDF assertions to verify a real download.
- Road Python tests: 5 passed; PowerShell recovery tests: 5 passed.
- Playwright local visual checks: Thai/English at desktop/mobile widths. Local configuration response was mocked because production CORS correctly rejected the dev origin.
- Playwright production checks at 390 and 1440 px: contact returned HTTP 200, OA destination matched the configured account, LIFF entry visible on mobile only, no horizontal overflow or page runtime errors. Website submission opened registration with `returnTo=/sites/new`. Production checks used the actual configuration response, without mocking.

## Backup verification

Local backup `backups/automated/rbc-20261106-101643.dump`, 870606822 bytes.

SHA-256: `7F27821F89EEB924411CCC5BF3966AA93354E53BEC671F1450CA82B79FC03DF0`.

Restore test passed: 11 users, 23 sites, 131 reports, 1203 metrics. This verifies a local backup, not an off-host copy.

## Remaining verification and configuration

- Actual customer OTP completion and a new complete production submission/report flow remain unfinished. An OTP request succeeded, but that does not prove account verification or the entire customer flow.
- Actual LINE identity, submission and tracking inside the mobile LINE app still require device verification. Navigation tests mock the SDK; public browser checks verify entry links only.
- Staff notification recipient is not paired. Customer OA entry and staff notifications are separate concerns.
- New multi-region production analysis timings have not yet been collected in this release session.
- Off-host backup destination and outage notification recipient remain unconfigured.
- These results do not establish that every production path has been tested.
