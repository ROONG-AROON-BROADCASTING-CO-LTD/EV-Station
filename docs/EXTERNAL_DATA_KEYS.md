# External data API keys

The API server reads these values from the root `.env` file. Keys are never
compiled into the web app and must not use a `VITE_` name.

```dotenv
GOOGLE_MAPS_SERVER_API_KEY=
OPEN_CHARGE_MAP_API_KEY=
GISTDA_API_KEY=
```

## Google Places API (New)

1. In the existing Google Cloud project, enable **Places API (New)**.
2. Create a separate key named `RBC Server – Places`.
3. Restrict that key to **Places API (New)** only. Do not use it in a browser,
   LIFF, or a `VITE_` variable.
4. Put it in `GOOGLE_MAPS_SERVER_API_KEY`.

The server requests only these fields: place ID, display name, primary type and
coordinates. It queries selected place types around the submitted site. If a
type reaches Google's 20-result Nearby Search cap, that evidence stays visible
but is excluded from the POI score instead of being undercounted.

## Open Charge Map

1. Request an API key from Open Charge Map.
2. Put it in `OPEN_CHARGE_MAP_API_KEY`.

It is supplemental competition evidence. It is merged with OpenStreetMap and
the connected provincial datasets, with location/name deduplication. A zero
result never proves there are no competing stations.

## GISTDA Elevation

For flood evidence, configure **`GISTDA_GI_SERVICE_API_KEY`** separately in
the API server `.env`, then recreate the API service to load it. Register the
GI Service credential at `https://api-gateway.gistda.or.th/v2`. Use the linked
public Gateway v1.1 contract (`api_key`, `lat`, `lon`). Live verification on
12 October 2026 confirmed that the Gateway credential must use `api_key`;
the upstream `key` parameter is not interchangeable. The application reports a missing connection until the
credential is present; Sphere elevation keys are never substituted.
Do not paste credentials into chat or commit `.env`.

The separate newer Disaster Platform key is `GISTDA_DISASTER_API_KEY`.
Its 1/3/7/30-day connector additionally requires `GISTDA_DISASTER_ENABLED=true`
and confirmation of reuse terms. It is disabled by default.

1. Obtain an API key for GISTDA Sphere Elevation.
2. Put it in `GISTDA_API_KEY`.

This only adds preliminary terrain/site-preparation evidence. It does not
confirm drainage, bearing capacity, earthwork design, or construction cost.

## Apply changes

After editing `.env`, run:

```powershell
docker compose up -d --build api
```

The startup script runs the equivalent startup sequence on the next launch.
