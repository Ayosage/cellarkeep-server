# CellarKeep rewrite: Go server, Next.js client, device API

Date: 2026-09-12
Status: approved in chat, awaiting written review
Supersedes: `2026-08-28-cellarkeep-production-tracker-design.md` for
architecture and data ownership. The product definition in that spec
(event journal, recipes, guided schedule, inventory, thin bottle
inventory) carries over unchanged.

## 1. Why

Two reasons, in priority order.

1. Brandon needs a portfolio project that is not exclusively
   JavaScript. The data layer, domain logic, and API move to Go.
2. CellarKeep needs to hold the lab metrics a winemaker actually
   tracks (pH, titratable acidity, free and total SO2, malic acid,
   Brix, density, ABV, temperature) and take readings from devices in
   the cellar. Today only gravity, temperature, and pH reach the UI,
   and there is no device path at all.

## 2. System shape

Three repositories with independent release cadences.

| Repo | Language | Owns |
|---|---|---|
| `cellarkeep-server` (new) | Go | Postgres schema, domain logic, auth, the HTTP API, the OpenAPI contract, the self-host compose file, the Docker image |
| `cellarkeep` (existing) | TypeScript, Next.js | The web client only. Generated API client, pages, components, calculators, Playwright |
| `cellarkeep-pi` (later) | Go | Headless daemon on a Raspberry Pi. Not built in this milestone |

### The contract

`api/openapi.yaml` in the server repo is the single source of truth.
It generates the Go server interface (`oapi-codegen`) and the
TypeScript client (`openapi-typescript` + `openapi-fetch`).

- All routes live under `/api/v1`.
- The server serves its own spec at `/api/v1/openapi.json`.
- The spec carries `info.version` as semver. Additive changes bump
  minor. Breaking changes bump the path to `/api/v2` and the major.
- Each client pins the spec version it was generated from and checks
  the server's major at startup. A mismatch is a visible error, not a
  silent failure.

### Deployment

Self-host-first and free-tier only, as before. The server repo ships
a `docker-compose.yml` with the server image and Postgres. That
compose file is the entire install. Brandon's public demo instance on
Fly is a follow-up after this milestone.

## 3. Data model

Migrations start fresh. Nothing is deployed, so no data migrates from
the current Drizzle schema.

### Carried over unchanged in shape

`users`, `sessions`, `invites`, `vessels`, `ingredients`,
`inventory_items`, `recipes`, `recipe_ingredients`, `recipe_steps`,
`batches`, `batch_events`, `bottle_lots`, `tasting_notes`.

The event journal keeps its design: one `batch_events` table with a
type discriminator, `planned` / `done` / `skipped` status, and a JSON
payload validated per type. Recipes stamp planned events with
`START+n` and `PREV+n` anchors. Inventory deducts only on a confirmed
done addition. Vessel occupancy derives from racking events. Editing a
recipe never mutates in-flight batches.

### New: `readings`

One row per metric per measurement. Sensor readings and bench values
are the same table.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid | |
| `metric` | enum | from the registry below |
| `value` | double | validated against the metric's range |
| `measured_at` | timestamptz | |
| `note` | text, nullable | |
| `vessel_id` | uuid, nullable | at least one of `vessel_id` / `batch_id` is set |
| `batch_id` | uuid, nullable | denormalized, see attribution |
| `source` | enum `manual` / `device` | |
| `device_id` | uuid, nullable | set when `source = device` |
| `client_id` | text, nullable | idempotency key, unique per `(device_id, client_id)` |
| `event_id` | uuid, nullable | the planned measurement event this reading fulfilled |
| `created_at` | timestamptz | |

Indexes: `(batch_id, metric, measured_at desc)`,
`(vessel_id, measured_at desc)`, unique `(device_id, client_id)`
where both non-null.

The `measurement` event type stays as a schedule item ("take a
gravity reading on day 3"). Its payload no longer holds values. When a
user logs readings against a planned measurement event, the readings
carry `event_id` and the event transitions to `done`.

### Metric registry

A fixed Go table, mirrored by a Postgres enum. Each entry has a
display name, unit, decimal precision, and a validation range.

| metric | unit | range |
|---|---|---|
| `gravity` | SG | 0.900 to 1.200 |
| `brix` | °Bx | 0 to 50 |
| `density` | g/cm³ | 0.9 to 1.2 |
| `ph` | pH | 2.0 to 5.0 |
| `ta` | g/L tartaric | 0 to 20 |
| `free_so2` | mg/L | 0 to 200 |
| `total_so2` | mg/L | 0 to 400 |
| `malic_acid` | g/L | 0 to 10 |
| `abv` | % v/v | 0 to 25 |
| `temp` | °C | -10 to 60 |
| `humidity` | %RH | 0 to 100 |

ABV is stored when an instrument measures it. When a batch has no
measured ABV, the client displays the value derived from original
gravity and the latest gravity reading, labelled as derived.

Adding a metric is one registry row, one enum migration, and one
line in the OpenAPI enum.

### Batch attribution

A device reading names a vessel, never a batch. On insert the server
resolves which batch occupied that vessel at `measured_at` from
racking events and stores it in `batch_id`. If no batch occupied the
vessel, `batch_id` stays null and the reading is still kept as a
vessel reading. A manual reading from the web names the batch
directly, and `vessel_id` is filled from the batch's current vessel.

### New: `devices`

| Column | Type |
|---|---|
| `id` | uuid |
| `name` | text |
| `token_hash` | text |
| `created_by` | uuid, user |
| `created_at` | timestamptz |
| `last_seen_at` | timestamptz, nullable |
| `revoked_at` | timestamptz, nullable |

Source configuration (which probe, which tank, what interval) lives
on the Pi in the later milestone. The server needs only who posted
and for which vessel.

## 4. API

JSON over HTTP, cursor-paginated lists, errors as
`application/problem+json`.

### Web auth

- `POST /auth/setup`: creates the first user and makes them admin.
  Returns 409 once any user exists.
- `POST /auth/login`, `POST /auth/logout`, `GET /auth/me`.
- `POST /auth/register`: requires a live invite token. Sign-up is
  otherwise closed.
- Passwords hash with argon2id.
- Session cookie: `HttpOnly`, `Secure`, `SameSite=Lax`, 30-day
  rolling expiry. All sessions for a user drop on password change.
- CSRF: every state-changing request must carry an `Origin` header
  equal to `CLIENT_ORIGIN`. Requests without a matching origin are
  rejected with 403.
- Rate limits on login per IP and per email. Lockout after repeated
  failed logins.
- Login responses are byte-identical for known and unknown emails.
- No password reset in this milestone. The app sends no email, and
  the current app has none either. An admin can revoke a member's
  sessions and issue a fresh invite; self-service reset waits for an
  email path.

### Device auth

- `POST /devices` (admin) creates a device and returns the plaintext
  token exactly once. Only the hash is stored.
- Devices send `Authorization: Bearer <token>`.
- Device tokens are scoped to `POST /readings` and `GET /vessels`.
  Any other route returns 403.
- Every accepted request updates `last_seen_at`.
- `POST /devices/{id}/revoke` sets `revoked_at`; the token stops
  working immediately.

### Resources

- `vessels`, `ingredients`, `inventory`, `recipes`, `batches`,
  `bottle-lots`, `tasting-notes`, `invites`, `devices`: standard
  list, get, create, update, and delete where the current app allows
  it.
- `batches/{id}/events`: list, create (freeform log), update, and the
  transitions `done`, `skip`, `reschedule`, matching the current app. `done` on an addition deducts inventory in the
  same transaction.
- `batches/{id}/start` stamps the recipe's planned events.
- `today`: planned events due, grouped by batch.
- `readings`:
  - `GET /readings?metric=&vessel=&batch=&from=&to=&cursor=`
  - `GET /batches/{id}/readings/latest`: one row per metric.
  - `POST /readings`: accepts an array. Each item may carry
    `client_id`. Items whose `(device_id, client_id)` already exist
    are skipped and reported in the response as `duplicate`. The
    response lists each item's outcome so a client can retry safely.
    Manual posts from the web may also group several metrics under one
    `measured_at` and one `event_id`.

### Live updates

The client polls in this milestone. An SSE endpoint is a later
additive change.

### Calculators

Stay in the Next client as pure TypeScript. They read no data.

## 5. Go server internals

```
cmd/cellarkeep/        one binary: serve, migrate, seed --demo
api/openapi.yaml       the contract
internal/api/          generated server interface + one handler file per resource
internal/auth/         argon2id, sessions, device tokens, middleware
internal/domain/       pure logic, no database imports
internal/store/        sqlc-generated queries over pgx, transaction helpers
internal/migrations/   goose SQL files, embedded, applied on boot
```

Dependencies: `chi`, `pgx`, `sqlc`, `goose`, `oapi-codegen`,
`golang.org/x/crypto/argon2`, `slog` from the standard library.

### Layers

Handlers decode and validate against the spec, call a domain service,
encode the result. Domain holds the logic that is pure TypeScript
today: the schedule engine with its anchors and drift handling, event
lifecycle rules, vessel occupancy, inventory deduction, the metric
registry, and batch attribution for readings. The store is generated
SQL plus a transaction helper. Domain never imports the store.

### Testing

- Domain: table-driven unit tests, no database. The existing
  TypeScript tests for the schedule engine and calculators translate
  one to one where the logic moves.
- Store and handlers: against a real Postgres. Locally that is the
  compose `db-test` service on host port 5433 (5432 belongs to the
  local Homebrew Postgres). Schema migrates once per run; each test
  runs in a transaction that rolls back.
- One HTTP end-to-end test walks the happy path: setup, vessel,
  recipe, start batch, complete a step, log readings, bottle, drink.
- CI: `go vet`, `golangci-lint`, unit tests, then database tests
  against a Postgres service container. A green `main` publishes the
  Docker image to the GitHub container registry tagged with the
  commit and the spec version.

### Operations

- Config is environment only: `DATABASE_URL`, `SESSION_SECRET`,
  `CLIENT_ORIGIN`, `ADDR`. Missing values fail at boot.
- Request bodies cap at 1 MB. Read, write, and idle timeouts are set.
- `slog` structured logs. Security events at warn: failed logins,
  lockouts, rejected origins, rate-limit hits, bad or revoked device
  tokens.
- `GET /healthz` pings the database.
- TLS and HSTS terminate at the reverse proxy in front of the server;
  the compose file documents that expectation.

## 6. Next.js client

### Removed

Drizzle and the schema, migrations, Better Auth, the seed script, the
db test suite, and every server action that touches the database.

### Kept

Calculators, design tokens, components, motion utilities, the
Playwright happy path.

### Added

- `src/api/`: the generated client, regenerated when the pinned spec
  version changes. A CI check fails if the checked-in client is stale
  against the pinned spec.
- Server components and server actions call the Go server by
  `API_URL`, forwarding the session cookie and setting `Origin` to the
  client's own origin.
- Browser code goes through a Next rewrite of `/api/v1/*` to the Go
  server, so requests are same-origin and cookies need no CORS. Only
  the readings polling on the Lab screens uses this path.
- `proxy.ts` keeps its optimistic cookie check, keyed to the Go
  session cookie name.

### The Lab screens

**Batch Lab panel** on batch detail. One row per registry metric:
latest value with its unit, when it was taken, a sparkline of its
history, and a flag when the value is outside the metric's range or
older than a staleness threshold. Below the rows, a full history
table with source and note. A "Log readings" form enters several
metrics for one `measured_at` in one submit and posts one array. When
opened from a planned measurement event, it carries that `event_id`.

**Cellar Lab overview** at `/lab`. A grid of active batches by
metric with the latest value in each cell. Stale cells dim,
out-of-range cells are marked, empty cells stay empty. Per-vessel
temperature from devices appears in the same grid once the Pi
exists. This is the "what needs attention" view.

**Devices** under `/settings`. Create a device, show its token once
with a copy control, list devices with last-seen times, revoke.

The existing gravity chart generalizes into one metric chart used by
the sparklines and the batch detail chart.

### E2E

Playwright runs against the real server. Compose pulls the server
image from the GitHub container registry at the pinned tag plus
Postgres. Port 3100 for the client under test, as today.

## 7. Scope and order

1. **Server core.** Repo, contract for auth, vessels, ingredients,
   inventory, recipes, batches, events, today, bottle lots, tasting
   notes, invites. Domain port with translated tests. Auth. CI and the
   published image.
2. **Server readings and devices.** Registry, `readings`,
   attribution, device tokens, idempotent ingest.
3. **Client rewire.** Strip the database layer, generate the client,
   repoint every page and action, Playwright green against the
   published image.
4. **Lab screens.** Batch Lab panel, cellar overview, Devices
   settings.

Two implementation plans, one per repo, written in that order.

### Out of scope

- The Pi daemon and its source configuration.
- A bench terminal UI on the Pi.
- The SSE stream.
- The v2 cellar module and CSV import.
- Data migration from the current schema.
- The public demo instance on Fly (follow-up after step 4).

## 8. Decisions log

- Go over Java: one static binary cross-compiles to the Pi, so the
  daemon and server share a language and types.
- REST with OpenAPI over Connect/gRPC: curl-able from a Pi shell,
  familiar browser tooling, no protobuf toolchain for readers.
- Separate repos over a monorepo: two independent maintenance queues,
  and the Go project stands on its own on GitHub.
- Readings as rows over JSON payloads: one query path for sensors and
  bench values, and the UI can render every metric without per-field
  plumbing.
- Idempotent ingest designed now, not with the Pi: the server contract
  should not change when the daemon arrives.
- Calculators stay client-side: they read no data, and a round trip
  per keystroke is worse for no gain.
