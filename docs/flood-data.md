# Flood evidence in RBC site analysis

Historical, recent observed, and forecast evidence are different. Recent observations never increase a historical year count or directly change the investment score. Missing records never mean flood-free conditions. Road and parcel flooding cannot be inferred from district reports or proximity alone.

## Sources verified on 5 October 2026

- DDPM district history: existing embedded district records remain the historical scoring input where an exact district is matched. Provincial annual summaries are context, not district years.
- [GISTDA Disaster](https://data.go.th/dataset/flood-disaster-data): observed polygon windows of 1, 3, 7 and 30 days. The adapter uses the [official API contract](https://disaster.gistda.or.th/v2/services/open-api), an `API-Key` header, a site-centred bounding box, and polygon containment at the site pin. A bounding-box match does not confirm access-road flooding. Dataset reuse terms are not specified in the catalogue; observations are context only.
- [GISTDA historic recurrence](https://opendata.gistda.or.th/th/dataset/disasters-01): catalogue describes 2011–2023 and registration. The older ArcGIS connector covers **2011–2020 only**, is optional, and must not be labelled 2026 coverage. It defaults to disabled.
- [FloodCheck](https://data.go.th/dataset/floodcheck): supporting weather, water-level and road-gauge source. Live unauthenticated probes returned HTTP 403. No working connector or road-level coverage is claimed.

## Configuration and failure handling

### GI Service point queries added on 6 October 2026

The registered credential goes in `GISTDA_GI_SERVICE_API_KEY`. The default
base URL is `https://api-gateway.gistda.or.th/api/2.0/resources/gi-service/v1.1/disasters`.
Live public Gateway verification on 12 October 2026 confirmed that the user
credential must use `api_key` with `lat`/`lon`. The `key` shown in upstream
examples is not the public Gateway credential. See
[official collection](https://documenter.getpostman.com/view/21652831/2sBXcKCyhC).
Never reuse Sphere keys or copy demo credentials. Keys are absent from result
JSON, references and cache names; transport errors are reduced to safe codes.

- [Recurrence](https://data.go.th/dataset/disasters-01): Open Data Common;
  all 13 annual entries for 2011–2023 are validated before accepting a record.
- Legacy [one-day point extent](https://data.go.th/dataset/disasters-03): Open Data Common;
  observation dates are preserved separately from retrieval dates. Records
  older than 72 hours are visibly stale. A negative response without a source
  date is labelled accordingly, never proof of current flood-free conditions.
- On 12 October 2026, the legacy `flood-extent-1day` service returned 404 in
  both server requests and the official Gateway preview. The default one-day
  reader now uses the documented `/features/flood/1day` FeatureCollection
  service with the registered Gateway `api_key`. It checks polygon containment
  locally; bbox filtering is not assumed to work. Truncated negative results
  remain missing. A positive overlap can be shown even in a partial collection.
  Response links containing the credential are redacted before caching.
  This source has its own reference and unspecified catalogue licence, context
  only. Collection `timeStamp` is never treated as satellite observation time.
  Observation dates remain unknown until a documented acquisition field is
  available. The latest response remains excluded from historical scoring.
- HTTP errors, missing keys, malformed or inconsistent records stay missing.
  Historical and latest queries fail independently. Valid responses are cached
  for 24 hours / one hour with the original retrieval timestamp retained.

The newer 1/3/7/30-day Platform connector is now opt-in through
`GISTDA_DISASTER_ENABLED=true`, after confirming its reuse terms. The older
ArcGIS reader remains optional. LDD CC BY-NC-ND and HII CC BY-NC sources are
not imported. No nationwide daily archive or scheduled collector is claimed.

Set `GISTDA_DISASTER_API_KEY` securely in the server environment using your registered Disaster credential. `GISTDA_API_KEY` is the Sphere elevation credential and is intentionally not reused. `GISTDA_DISASTER_BASE_URL` defaults to the official resources API. Never copy demo keys from a catalogue.

Each recent window has its own available/missing status. A 401/403, 429, timeout, malformed JSON, or absent feature collection is unavailable, not an empty result. Partial responses are labelled; only supported GeoJSON Polygon/MultiPolygon collections are accepted. Results are cached for one hour, preserving the original retrieval timestamp. Retrieval time is not satellite acquisition time. New analysis runs persist the evidence with their results; existing reports need a new analysis to fetch it.

This change does not establish complete national annual history through 2026, archive daily national polygons, register an API account, or create a scheduled automation. The pasted research text is source material, including its old scheduling claim, not proof that such a job exists.
