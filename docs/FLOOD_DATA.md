# Flood screening: data scope and refresh

The API uses an embedded, reproducible DDPM snapshot by default. This removes
historical-data downloads from the analysis request. Address-to-district lookup
still uses the configured reverse-geocoding provider and its cache.

## District history used in scoring

The `preliminary-v5` screen prefers **complete positive GISTDA GI Service
point history (2011–2023)** when configured. The linked official Postman
contract returns `total` and `detail[{year,freq}]`. The adapter requires all
13 unique years and checks totals before accepting the record. The heuristic
is `100 × (1 − number of years with positive observations / 13)`.
It is not a calibrated flood probability. Empty/malformed results or all-zero
history do not produce a safe-site score; existing DDPM history remains the
fallback. Current one-day observations never modify historical scoring.
The selected score rule explains which source and period were used. Both
sources remain in the persisted report evidence, without adding their years
together. Credential failure does not erase usable DDPM district evidence.

- Source: DDPM GD015, both public CSV parts, B.E. 2562–2567 (2019–2024).
- Summarized by province and district, with deduplicated source rows.
- 909 district-name records have at least one reported year in the imported data.
- These are administrative reports, not a complete inventory of land parcels.
- A matched district score is `max(0, 100 - 10 × reported years)`.
- No matched district record means missing evidence, not a 100-point flood score.
- The rule is an uncalibrated screening heuristic; it is not a flood probability.

## Latest provincial context: B.E. 2568 (2025)

### GISTDA published annual archives through B.E. 2569 (2026)

On 12 October 2026, the authenticated public STAC catalogue exposed yearly
collections for 2011–2026. RBC now checks those collections, as well as the
1/3/7/30-day Features feeds and the recurrence map's published annual columns
(currently through 2024). This extends available evidence, not the claim that
every flood event or every road is recorded.

- Yearly archive records are administrative flood summaries. The reader uses
  an amphoe code when a province/district name pair can be matched, otherwise
  a local bounding box. It checks numeric province/district IDs in returned
  rows; some Thai labels in the 2024/2025 source are encoding-corrupted.
- A small embedded name/code registry comes from
  [kongvut/thai-province-data](https://github.com/kongvut/thai-province-data),
  with 77 province names and 930 district-name records in the retrieved
  snapshot. The MIT notice is retained beside the snapshot. This is a code
  lookup, not a national boundary resolver or proof of all current boundaries.
- STAC item footprints were stale in live checks: the 2026 item index omitted
  Bangkok although its data asset contains Bangkok records. Therefore item
  discovery does not use those footprints to rule out a location; geographic
  filtering is applied to the actual data assets.
- Administrative areas are deduplicated within each year. Counts are not
  incident counts. `area_rai` is treated as rai only in the explicit legacy
  year schema; `flood_area` values retain unspecified units. No unit is guessed.
- Pagination is followed within bounded request/body limits. Truncated pages,
  repeated pages, failed sources and unsupported schemas stay visibly partial
  or unavailable. An empty one-day feed never becomes a flood-free finding.
- Product-name dates from recent feeds are explicitly inferred. Response
  timestamps and ingestion dates are not satellite acquisition dates.
- The current calendar year is marked partial. Annual administrative totals
  and overlapping recent windows are not double-counted into the existing
  point-history score. The UI labels that score as historical screening.

Live provider verification at an Ayutthaya sample resolved district code 1401,
queried all 16 annual collections, and found 14 administrative areas with
reports in 2026. Source-scope matching, legacy/current schemas, repeated-page
handling and the current-year UI were also tested. These sample checks do not
establish exhaustive coverage throughout Thailand.

Source: [DDPM annual flood statistics GD027](https://catalog.disaster.go.th/dataset/dpm-gd027),
resource `9eae087c-8931-4a74-9968-200cdb3d2fb3` (`untitled.xlsx`).

Inspection of the actual workbook found 74 reported province rows, regional
subtotals and a national total. The columns named **District**, **Sub-district**
and **Community** contain **counts**, not location names. **Time** contains the
number of reported occurrences, not a timestamp. The catalog's short English
data dictionary does not convey these distinctions.

Only province rows for flood disaster type are imported. Headers and totals are
excluded. The snapshot preserves reported occurrences, affected administrative
area counts, people and households. Missing provinces remain absent; zeros are
retained exactly as reported and do not establish no risk. These counts are
reported totals, not independently deduplicated people, events or households.

This provincial summary appears alongside the district history but **does not
extend the district period to 2568 and does not change the district score**.
When no district match exists, provincial context can still be shown while the
flood metric remains missing and is excluded from scoring.

The 2569 (2026) partial-year situation figures in the user's PDF are not mixed
into completed annual district history.

## Source licence review

Both DDPM catalog packages state **Open Data Common**. The importer verifies
this metadata before writing a new snapshot and stores exact resource URLs,
download SHA-256, source update metadata and import time in the generated JSON.

The HII [Flood Mark](https://data.go.th/dataset/flood-mark) and
[flood-area](https://data.go.th/dataset/flood-area) catalog pages state
**Creative Commons Attribution Non-Commercial**. They were not added to the RBC
commercial screening pipeline. GISTDA's old optional flood-layer reader has no
confirmed licence recorded here; it remains outside the default provider list
and is not the source of the new district-history result.

## Refresh

Run `scripts/import-ddpm-flood.py` with Python and `openpyxl`, then run API and
frontend tests and rebuild the API image. The script downloads public source
files, validates their current schemas, and writes
`apps/api/internal/provider/data/ddpm_flood_snapshot.json`. It stops if the
schema, licence or expected 2568 row count changes. Review those changes before
updating the script. Raw workbook/CSV downloads are not stored in PostgreSQL.

Optional `DPM_FLOOD_HISTORY_CSV_URLS` (or the legacy singular variable) retains
the live CSV override, restricted to the full GD015 schema for 2562–2567. An
annual provincial workbook is not a valid replacement URL. Default configuration
leaves the override empty and uses the embedded snapshot.

## Production verification — 13 October 2026

The persisted Pracha Uthit report `fdd10dc7-ef15-41ff-a746-a52bedac6f50`
(created 12 October 2026, 17:59 Bangkok time) was verified after restarting
Docker Desktop. All 16 annual results (2011–2026), all four recent windows
(1/3/7/30 days), and district-filtered mapped history remained in PostgreSQL
and in the public report after reload. Overall screening score remained 94.68.
The annual disclosure expands successfully. TH/EN source-period explanations
were checked, and a 390px browser viewport had no horizontal page overflow.
This is browser viewport testing, not a physical-device check.

This sample resolves to Huai Khwang district 1017. The mapped-history response
contains 18 district records with observed 2011 evidence and zero pin overlaps;
the 2011 annual collection contains two administrative areas with reports.
The 2026 annual search and recent windows returned no published records for
this sample. They remain unverified conditions, not a flood-free conclusion.
The original source retrieval dates are preserved; opening the saved report
does not refresh the upstream evidence. The current year is partial, and
these checks do not establish complete national event coverage.
