# Vercel preview and nationwide screening check

Use `apps/web` as the Vercel project root. The frontend build uses a pinned pnpm version and checks the generated Thai and English pages. The preview serves static files and forwards `/api/v1/` to the existing RBC API. The Go API, its worker, PostgreSQL and Redis continue to run on their existing host.

Deploy an isolated preview with `vercel deploy` from the linked frontend directory. Check both public languages, private route fallback, an unknown URL returning 404, the public LIFF configuration endpoint and a protected API request without a session. Preview responses carry `X-Robots-Tag: noindex, nofollow`. The same-origin request transform removes the browser Origin header only when browser Fetch Metadata reports a same-origin request, so the forwarded request can reach the upstream API without changing its CORS allowlist. Backend bearer-token authentication still applies. Verify browser authentication, upload, and LIFF on the target host before any cutover.

## Nationwide screening check on 6 November 2026

The audit used one sample coordinate per province, a 3 km main radius, live providers and the current rules from `ee6a3d5`. It saved test sites and reports in memory, used production PostGIS and Redis for reads, and did not send uncached Google Places requests. It did not exercise the HTTP queue or the LINE mobile app.

- All 77 pins resolved to the intended province, completed analysis and produced a score using 7 of 9 scoring factors (79.17% of the scoring weight).
- All 77 used local road data. Official AADT matched 76 pins. Pracha Uthit used road-based estimated traffic.
- GISTDA returned a usable 2569 annual result for 76 pins and records within the searched scope for 40. Suphan Buri returned an invalid response for that year. Annual results describe the source search scope and do not establish plot-specific flooding.
- Median runtime was 4.73 seconds, p95 9.00 seconds, and maximum 15.40 seconds. These timings exclude the production queue.
- POI was unavailable in this cache-only Google Places run. Site requirements did not contribute a score. Each result applies to its sample pin and radius.

Per-pin CSV and raw results are stored in the local ignored audit directory.
