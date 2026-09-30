# Flood screening: data scope and refresh

The API uses an embedded, reproducible DDPM snapshot by default. This removes
historical-data downloads from the analysis request. Address-to-district lookup
still uses the configured reverse-geocoding provider and its cache.

## District history used in scoring

- Source: DDPM GD015, both public CSV parts, B.E. 2562–2567 (2019–2024).
- Summarized by province and district, with deduplicated source rows.
- 909 district-name records have at least one reported year in the imported data.
- These are administrative reports, not a complete inventory of land parcels.
- A matched district score is `max(0, 100 - 10 × reported years)`.
- No matched district record means missing evidence, not a 100-point flood score.
- The rule is an uncalibrated screening heuristic; it is not a flood probability.

## Latest provincial context: B.E. 2568 (2025)

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
