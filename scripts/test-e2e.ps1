$ErrorActionPreference = 'Stop'

try {
  docker compose -p rbc-ev-station-e2e -f docker-compose.e2e.yml up --build --wait
  if ($LASTEXITCODE -ne 0) { throw "Isolated Docker services failed to start." }
  docker compose -p rbc-ev-station-e2e -f docker-compose.e2e.yml exec -T db-e2e psql -U rbc_e2e -d rbc_ev_station_e2e -c "INSERT INTO users (email, password_hash, display_name, role, is_active) VALUES ('owner@e2e.local', crypt('E2e-password-123!', gen_salt('bf')), 'E2E Owner', 'super_admin', TRUE) ON CONFLICT (email) DO NOTHING;"
  if ($LASTEXITCODE -ne 0) { throw "Isolated test account could not be created." }
  pnpm --filter @rbc/web test:e2e
  if ($LASTEXITCODE -ne 0) { throw "Playwright end-to-end tests failed." }
}
finally {
  docker compose -p rbc-ev-station-e2e -f docker-compose.e2e.yml down --volumes --remove-orphans
}
