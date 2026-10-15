# Isolated Playwright environment

Run `npm run test:e2e` from the repository root. The script starts a dedicated
Docker Compose project named `rbc-ev-station-e2e`, then always removes its
containers and test-only volumes after Playwright exits.

The environment uses these isolated dependencies:

- PostgreSQL database `rbc_ev_station_e2e` with the `rbc_e2e` user
- Redis service `redis-e2e`
- MailHog SMTP and API, exposed only at `127.0.0.1:18025`
- API exposed only at `127.0.0.1:18080`

The configuration never reads the normal Compose database credentials or the
repository `.env` file. Do not point `DATABASE_URL`, `SMTP_HOST`, or
`VITE_API_BASE_URL` in this workflow at production services.

## Functional regression coverage (15 October 2026)

The suite runs seven scenarios on Chromium desktop and Pixel 7 viewport/touch
emulation (14 cases). It uses a real isolated API, PostgreSQL, Redis and SMTP
mailbox. It does not create production customers or send real messages.

- Owner creates a site, uploads a decodable PNG and complete PDF, reloads,
  edits with draft recovery, analyzes, and reopens the persisted report.
- Ten distinct PDFs survive reload. Replacing the selected file group does not
  count the old selection twice. Replaying an upload at the quota succeeds;
  an eleventh distinct file is rejected without removing existing evidence.
- Lost create/upload responses are simulated **after** the API accepts them.
  Retry, including a browser reload and file reselection, preserves one site and
  one stored copy of each file. A rejected upload can also be retried.
- Offline submission waits for connectivity. An expired token opens sign-in,
  restores the original private route and account-scoped form draft, and allows
  completion without retyping. Drafts expire after 24 hours in tab storage;
  file bytes and credentials are never put in the draft.
- Sales creates and analyzes assigned sites. Customer registration uses a real
  OTP in MailHog; customer access to another user's site, team administration,
  and analysis initiation is rejected by the server. The customer can read an
  assigned report. The current view role is `customer`, not legacy `viewer`.
- PDF export invokes browser print. Chromium print output is inspected using
  three report variants: missing evidence, synthetic long assumptions, and
  a long Thai site name/customer report. The layout supports repeated table
  headers, stable column widths and multi-page text.

Artifacts and traces live under `%TEMP%/rbc-functional-qa`, outside Git. The
report variants are QA fixtures, not investment assessments. Providers are
intentionally unavailable in this environment: these tests verify graceful
missing-data behavior and persistence, not live GISTDA/Gemini coverage.

This does not certify physical iPhone/Safari behavior, native mobile file
pickers, the operating system's Save PDF dialog, or load/security readiness.

## Validated release

On 15 October, all 14 browser cases, 27 frontend unit/component tests,
frontend lint/build, backend `go test ./...` and `go vet ./...` passed.
The API and web Docker images were rebuilt and deployed without schema or
credential changes. Public-domain smoke checks passed for `/`, `/investment`,
`/branches` and `/login` at 1440 and 390 pixels: HTTP 200, no horizontal
overflow or uncaught browser errors. API health returned 200; unauthenticated
site access returned 401. Full authenticated flows above ran in the isolated
environment, not against real customer records.

Rollback images are `ev-station-api:before-functional-qa-20261015` and
`ev-station-web:before-functional-qa-20261015`. Existing deployment settings,
database and road snapshots are retained.
