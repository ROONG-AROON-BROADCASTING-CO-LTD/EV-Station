# Isolated Playwright environment

Run `pnpm test:e2e` from the repository root. The script starts a dedicated
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
