# Production audit and timeout repair — 14 October 2026

## Deployed outcome

The monorepo Docker services were rebuilt and restarted with production mode,
authentication required, and a new server-only signing secret. Existing sessions
must sign in again. Passwords and customer records were not changed.
Database, Redis, API and web host ports now bind to loopback; the existing
Cloudflare tunnel remains the public entry point.

## Findings and repairs

- The supplied screenshot points to a saved 12 October report, not a live
  source request. The report page now identifies newer completed reports and
  links to them without changing historical evidence.
- Overpass live requests timed out on the primary and both configured mirrors.
  A minimal request also timed out from the Windows host. The system's cooldown
  previously still retried cooling endpoints, repeating delays. Cooling endpoints
  are now skipped until their retry time. The existing exact-query road snapshot
  fallback retains its seven-day limit, original retrieval time and estimated status.
  This does not restore external service availability for uncached locations.
- WorldPop previously discarded the upstream task identity on a report deadline.
  Accepted tasks are now cached for one hour and resumed on subsequent requests
  with the same polygon, year and resolution. Failed/expired tasks are removed;
  incomplete results never become population evidence.
- Browser API calls now have bounded deadlines (60 seconds for JSON; 120 seconds
  for uploads/downloads), including body reads. Proxy 408/504/524 responses map to
  a useful timeout message. Mutations are not automatically retried.
- A late 401 from an old session can no longer clear a newer login.
- Authentication reads current account activation and role on protected requests;
  disabling or demoting an account now affects previously issued tokens. Database
  failures return 503 rather than logging the user out as though the token expired.
- Production startup rejects missing/weak default signing secrets, disabled
  authentication and in-memory-only storage. HTTP header/body/idle/write deadlines
  and graceful shutdown were added. Request timing logs contain route templates,
  not tokens, query strings or submitted customer content.
- Docker frontend builds now use the committed lockfile with frozen resolution;
  the previous build could install different dependency versions from those tested.

## Verification

- Frontend: 27 unit/component tests passed; TypeScript/Vite and Docker builds passed.
- Backend: `go test ./...` and `go vet ./...` passed, including account-role changes,
  disabled tokens, WorldPop resumption and Overpass cooldown regression tests.
- Browser: old-report notification and navigation tested at 390 and 1440 pixels
  using controlled responses; live public-domain navigation also passed with no
  uncaught page errors. Browser plugin not available; authorized Playwright/Edge used.
- GISTDA live checks: annual 2026 and complete annual inventory through 2026 passed.
  Empty recent windows remain unknown; absence is never interpreted as flood safety.
- WorldPop live sample returned estimated population in about one second.
- Overpass cold live sample failed at its bounded 30-second budget. This is an
  outstanding upstream availability issue, not a passing source check.
- A new production Pracha Uthit analysis completed in 37.6 seconds:
  `542fcc10-3717-4fc4-a952-0e2039a12a44`, score 94.68, weighted coverage 79.17%.
  Road and traffic evidence are estimated, using retained source evidence when
  refresh is unavailable. This score is not a probability of investment success.
- PostgreSQL custom-format backup restored successfully into an isolated container
  with no network or published ports. Counts matched: 20 sites, 126 pre-audit
  reports, 1,153 metrics. The temporary restore container was removed.
- Public homepage returned 200; unauthenticated sites API returned 401. Production
  API health returned 200. Database/Redis published ports were verified loopback-only.

## Remaining production work

1. Replace previously shared/test account passwords through account administration.
   Secret rotation invalidated old tokens but did not change these passwords.
2. Add durable automated off-host backups and operator alerts, then periodically
   repeat restore testing. This audit verified one backup, not an ongoing schedule.
3. Implement and validate authentication abuse/rate controls at the application or
   trusted ingress. Do not infer this protection from TLS or authentication alone.
4. Capacity/load testing and independent security review remain outstanding.
5. For nationwide road coverage during prolonged Overpass outages, obtain a
   maintained local/regional OSM extract or a separately provisioned service.
   Do not promise every location has road evidence based on the successful cached
   Pracha Uthit sample. Public instances are shared infrastructure:
   <https://wiki.openstreetmap.org/wiki/Overpass_API#Public_Overpass_API_instances>.

## Rollback material

Previous images: `ev-station-api:before-readiness-20261014` and
`ev-station-web:before-readiness-20261014`. Private environment backup and database
dump are retained in the operator's local temporary directory, outside Git.
Do not restore the weaker signing secret just to avoid reauthentication.
Prefer rolling back the application image while retaining hardened runtime settings.
No database schema changes were introduced by this repair.
