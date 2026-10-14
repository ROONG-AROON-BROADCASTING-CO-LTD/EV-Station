# Nationwide screening readiness — 12 October 2026

RBC accepts coordinate-based screening; coverage and score availability depend on
the evidence returned for that coordinate. Nationwide operation does not mean
every plot has every metric or a confirmed investment recommendation.

## Changes

- DDPM and DLT share a bounded administrative reverse lookup with in-flight
  request coalescing. DLT now uses the configured geocoder/cache/rate limiter.
- DLT province names are normalized against the existing 77-province inventory.
  A missing provincial row remains missing; it is never substituted with Bangkok.
- Current DLT downloads have a 15-second budget; optional previous-period data
  has a separate 3-second budget so a slow trend lookup does not consume the
  35-second composite collection deadline after current evidence is available.
- If only a province is resolved, DDPM can still show its published provincial
  summary. Provincial totals remain excluded from district-level flood scoring.
- Scoring v6 rejects missing-status persisted scores and absent EV/population
  numeric fields. Duplicate metric types cannot inflate weighted coverage.
- The report displays scored-factor count, weighted coverage, excluded factors,
  and follow-up guidance in Thai and English. Older reports can be recalculated.

## Existing data footprint

The embedded DDPM snapshot contains 909 district-name groups spanning 77
province names, for B.E. 2562–2567. The provincial B.E. 2568 snapshot has 74
records. These are published report inventories, not proof of exhaustive
district coverage or absence of flooding at unlisted sites.

Other existing connectors include DOH/DRR traffic, OSM roads, WorldPop,
Google Places, Open Charge Map, provincial charging records, and MEA/PEA.
Their availability and completeness must be assessed per result. Road class
is an estimated traffic indicator, not measured AADT. Grid proximity does not
confirm capacity or a connection point. No new source or paid service was added.

## Verification and remaining work

- Synthetic DLT collection exercised all 77 province names, including unavailable
  previous data. This does not validate geographic reverse matching in all 77.
- Live Nominatim checks returned HTTP 200 with the expected province and district
  for Chiang Mai, Khon Kaen, Bangkok, Chon Buri, Phuket and Narathiwat sample pins.
  Results are in `output/nationwide-live-geocoding.json`. These are geocoding-only
  checks, not complete live analyses of all providers.
- Browser checks exercised Thai/English, desktop/mobile, missing data and reload
  using mocked analysis responses. Build, provider/scoring tests and API tests
  are the local validation path.
- `GISTDA_GI_SERVICE_API_KEY` was configured in the ignored server `.env` on
  12 October 2026 and the API container was recreated. Live point requests to
  initial requests returned HTTP 407 because the Gateway credential was sent
  as `key`. Correcting the public parameter to `api_key` returned HTTP 200:
  Bangkok's sample returned `not found` (not a flood-free determination), and
  Ayutthaya's sample returned 13 annual records with a positive 2011 record.
  The legacy one-day endpoint returned 404, also reproduced in the authenticated
  GISTDA service preview. The reader was replaced with the documented
  `/features/flood/1day` service. An opt-in live test exercised the actual
  provider: history returned one observed year, latest returned a valid empty
  collection. Satellite observation dates were not supplied, so current
  conditions remain unconfirmed. Geometry, partial-result safety and cache
  credential redaction passed provider tests; frontend flood tests passed.
- API and web were rebuilt and deployed on 12 October 2026. Local API health
  returned 200; the public domain returned 200 with identical JS/CSS hashes to
  the deployed local web. Protected API routes returned 401 without a session.
  Existing database and Redis containers were retained. Previous images are
  tagged `before-nationwide-20261012` for rollback.
  Complete authenticated live regional analysis and external-service
  availability remain unverified by this deployment smoke check.
- Public APIs and reverse geocoding remain network dependencies. There is no
  offline national administrative-boundary resolver or complete 2026 flood archive.

Never widen the requested radius or substitute provincial counts for local
counts silently to fill a score. Report the scope, dates and missing evidence.

## All published flood years and recent windows — 12 October 2026

The flood adapter now checks all 16 published GISTDA annual archives for
2011–2026, recurrence-map columns through 2024, the original point-history
service, and all 1/3/7/30-day feeds. DDPM district history and its 2025 provincial
context remain independently displayed. The annual reader prefers an exact
province/district name-to-code lookup, validates numeric IDs from source rows,
and uses a bounding box only when a district code cannot be resolved.

Recent empty feeds are unknown conditions, not negative flood findings. Source
dates inferred from product filenames are labelled. Pagination and unsupported
schema failures remain visible. Annual administrative areas are deduplicated;
their counts are not event counts or plot-specific flood confirmations. Units
of the generic `flood_area` value remain unspecified, never silently converted
to rai. Current-year data is partial and is not double-counted into the existing
historical score.

The embedded public registry contains 77 province names and 930 district-name
records, with source fingerprints and the MIT notice. It is not a geographic
boundary dataset. See `docs/FLOOD_DATA.md` for source and schema details.

Live provider verification at an Ayutthaya sample retrieved all 16 annual
collections, including 14 administrative areas with reports in 2026 and 16 in
2025. The 2024/2025 encoding-corrupted Thai labels are handled by matching
numeric province/district IDs. Provider, scoring, API tests, vet and web build
passed. These checks establish the sample flow, not exhaustive countrywide
recording of every flood event.

## Pracha Uthit missing-score investigation — 12 October 2026

- The September report scored 91.47 under older evidence/rules. The October
  report `afc1033a-2d95-4e74-8170-6371cfc6dcb4` completed with 58.33% weighted
  coverage and no overall score. OSM road requests timed out at four seconds
  per mirror within an eight-second total budget. Backup services returned
  HTTP 500; Kumi and Private.coffee refer to the same service.
- OSM attempts now have ten seconds each within a shared thirty-second budget;
  the HTTP client allows twelve seconds. The composite deadline stays bounded.
- Successful, nonempty road responses now retain a seven-day snapshot keyed
  by the exact query (pin, radius, road selection). The original retrieval time
  is retained. Beyond the fresh-cache window, fallback road evidence is marked
  estimated and shown as cached, rather than silently presented as current.
  Different pins and snapshots older than seven days are rejected.
- During diagnosis, a successful HTTP 200 POST returned 531 road ways at
  07:25:58 UTC. This actual diagnostic response was imported into the matching
  cache with its original time and expiry, not copied from the old 91.47 score.
- After deployment, authenticated run `c5ff976c-a16e-4a03-aaaa-18765b226616`
  fetched fresh road data at 08:52:04 UTC: 526 mapped ways, nearest distance
  23.65 metres. The saved overall score is 94.68, with 7/9 scored factors and
  79.17% weighted coverage. This run did not depend on stale fallback evidence.
- Partial-evidence UI now states that data is insufficient for an overall
  score; investment advice remains pending instead of implying a low score.
  Provider/scoring tests passed, including expiry, pin isolation and timestamp
  preservation. API/web builds passed. Public-server availability remains a
  dependency; no guarantee of uninterrupted national coverage is claimed.
