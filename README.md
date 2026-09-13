# cellarkeep-server

Self-hosted production tracker for hobbyist wine, mead, and cider makers.
This is the Go server; the web client lives in `Ayosage/cellarkeep`.

## Self-host

Copy `.env.example` to `.env` and fill in real values, then:

```bash
docker compose --profile prod up
```

## Development

```bash
make db-up     # start dev and test Postgres containers
make test      # run tests that do not need a database
make test-db   # run the full test suite against TEST_DATABASE_URL
```

## API

The API is documented as an OpenAPI 3 document served at `/api/v1/openapi.json`.
