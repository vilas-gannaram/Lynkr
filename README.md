# Lynkr

Lynkr is a URL shortener built to learn Go fundamentals and system design by building something real, rather than following a toy tutorial. It's a plain Go backend (chi router + PostgreSQL via Supabase) with server-rendered Go templates for the UI.

Core features:
- Shorten a long URL into a short, random alias (`go-nanoid`-generated code), with an optional title
- Redirect `/{code}` to the target URL
- Track click counts per link (via a background stats upsert on redirect)
- Delete a link; its cache entry is evicted so it stops redirecting immediately
- JSON API under `/api/links` (create, list, delete)
- Server-rendered split-screen home page: shorten on the left, paginated links table on the right
- Redis cache-aside layer in front of the redirect lookup (optional — falls back to Postgres if unset/unreachable)

## 1. Repo structure

```
.
├── cmd/
│   └── server/            # the `main` package — the actual binary/entrypoint
│       ├── main.go        # wiring: env, db pool, Config struct, http.Server
│       ├── env.go          # Env: loads/validates DATABASE_URL, PORT from .env
│       ├── handlers.go     # Data + Handlers: DB deps + HTTP handler methods (HomePage, Redirect, CreateLink, ListLinks, DeleteLink)
│       ├── routes.go       # Routes: chi router setup, middleware, route table
│       ├── cache.go        # Cache: Redis cache-aside for code -> URL lookups (no-op if REDIS_URL unset)
│       └── helpers.go      # Codex: shared JSON read/write/error helpers
│
├── ui/
│   ├── ui.go               # embed.FS exposing html/ and static/ to the server
│   ├── html/
│   │   ├── layouts/        # base.gohtml — the shared page shell
│   │   ├── pages/          # home.gohtml — the home page template
│   │   └── partials/       # header.gohtml (fonts + stylesheet), footer.gohtml (currently unused)
│   └── static/              # script.js, styles.css served at /static/*
│
├── internal/
│   └── database/           # sqlc-generated code — do not hand-edit
│       ├── db.go            # Queries struct + DBTX interface
│       ├── models.go        # Go structs generated from sql/schema.sql
│       └── queries.sql.go   # typed Go functions generated from sql/queries.sql
│
├── sql/
│   ├── schema.sql          # table definitions (source of truth for the DB shape)
│   └── queries.sql         # hand-written SQL queries, annotated for sqlc
│
├── sqlc.yaml               # tells sqlc which files to read and where to generate code
├── .air.toml               # hot-reload config for local dev (air)
├── k6-load-testing/
│   ├── redirect-load-test.js  # k6 load test for the redirect endpoint
│   └── data.json           # saved GET /api/links response; its `code`s are the pool k6 hits
├── go.mod / go.sum
└── README.md
```

The rule of thumb: `cmd/` holds only `main` packages (binaries); `internal/` holds shared logic (like the DB layer) that isn't allowed to be imported outside this module.

Within `cmd/server`, the app is wired as a small dependency graph, all hanging off a top-level `Config` struct in `main.go`:

```
Config
├── Env      — loaded env vars (DATABASE_URL, PORT, REDIS_URL)
├── Data     — *database.Queries + *pgxpool.Pool
├── Cache    — Redis client for code -> URL lookups, wired into Handlers
├── Handlers — HTTP handler methods, depends on Data + Cache + Codex
├── Routes   — chi router, depends on Handlers
└── Codex    — JSON read/write/error helpers
```

## 2. sqlc — benefits & commands

**What it does:** sqlc reads plain SQL (`sql/schema.sql` for table shape, `sql/queries.sql` for queries annotated with `-- name: X :one/:many/:exec`) and generates type-safe Go code — no ORM, no reflection, no query builder. `internal/database/` is the generated output: `Queries.CreateURL(ctx, CreateURLParams{...})` is a real Go function with real Go types, and `models.go` is regenerated straight from `schema.sql`.

**Commands:**
```bash
# regenerate internal/database/* from sql/schema.sql + sql/queries.sql
sqlc generate

# validate SQL/config without generating (useful in CI)
sqlc vet

# check sqlc.yaml + SQL files for syntax errors
sqlc compile
```
Run `sqlc generate` any time you change `sql/schema.sql` or `sql/queries.sql`. If `sqlc` isn't on your `PATH` after `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`, it's likely sitting in `$(go env GOPATH)/bin` — add that to `PATH`, or call it directly from there.

### Database schema

Source of truth: `sql/schema.sql`. A link has at most one stats row, created lazily on its first click.

```mermaid
erDiagram
    links ||--o| link_stats : "has (ON DELETE CASCADE)"

    links {
        BIGSERIAL id PK
        TEXT code UK "NOT NULL"
        TEXT target_url "NOT NULL"
        TEXT title "NULL, 1-120 chars"
        TIMESTAMPTZ created_at "NOT NULL, default NOW()"
    }

    link_stats {
        BIGINT link_id PK, FK
        BIGINT click_count "NOT NULL, default 0"
        TIMESTAMPTZ last_clicked_at "NOT NULL, default NOW()"
    }
```

## 3. API

JSON endpoints live under `/api`; the redirect stays at the root so short links stay short.

| Method | Route | Description |
|---|---|---|
| `GET` | `/` | Home page (HTML) |
| `GET` | `/{code}` | `302` redirect to the target URL |
| `POST` | `/api/links` | Create a link. Body: `{"target_url": "https://...", "title": "optional, 1-120 chars"}`. Returns `202` with the short URL as a JSON string |
| `GET` | `/api/links` | List links, most recently clicked first. Query: `page` (default 1) and `limit` |
| `DELETE` | `/api/links/{code}` | Delete a link and evict it from the cache. `204` on success, `404` if the code doesn't exist |

Notes:
- `limit` must be 1-100 and defaults to 100; an out-of-range value currently falls back to 10 rather than being capped.
- Validation errors from `POST /api/links` come back as plain text, while body-parse and delete errors are JSON (`{"error": true, "message": "..."}`).
- `/ping` is a heartbeat endpoint.

## 4. Caching — Redis (cache-aside)

**What it does:** `cmd/server/cache.go` wraps a Redis client used only for the redirect lookup path (`code -> {id, target_url}`). `Redirect` checks the cache first and only queries Postgres on a miss; `CreateLink` writes through to the cache on creation so the first redirect doesn't miss, and `DeleteLink` evicts the entry. Entries expire after 24h (`urlCacheTTL`).

**Why this path:** redirects are the hottest, most read-skewed request in the app, and the mapping is effectively immutable once created, so it's the highest-value thing to cache. Click-count writes and the `/api/links` listing are intentionally left uncached.

**Configuration:** set `REDIS_URL` (e.g. `redis://user:password@host:port`). If it's unset, or Redis can't be reached at startup, the app logs a warning and runs with caching disabled — nothing else changes. This project is deployed on Render, whose managed Redis-compatible offering is called "Key Value".

## 5. System performance — load testing

`k6-load-testing/redirect-load-test.js` load-tests the redirect path (`GET /{code}`) against the deployed instance, picking random codes from `k6-load-testing/data.json` (a saved `GET /api/links` response). The checked-in script runs 100 VUs / 10,000 iterations; the 10 VU / 100 iteration screenshots below came from earlier, smaller runs. Refresh the pool with `curl 'http://localhost:8080/api/links?limit=100' > k6-load-testing/data.json` and run `k6 run k6-load-testing/redirect-load-test.js`. Two things worth distinguishing when reading the results:

- **End-to-end (end goal) latency** — following the redirect through to the original page. This measures what a real user experiences, but folds in the destination site's own latency/availability, which has nothing to do with Lynkr.
- **System latency** — measuring Lynkr's own redirect response only (`redirects: 0` in the k6 script), isolating cache/DB lookup time from the app itself.

**End-to-end (following redirects to the destination page):**

![End-to-end load test results](k6-load-testing/eval-endgoal-10vu-100itr.png)

**System (Lynkr's redirect response only, `redirects: 0`):**

![System load test results](k6-load-testing/eval-system-10vu-100itr.png)

**System, 100 VUs / 10,000 iterations:**

![System load test results at 100 VUs](k6-load-testing/eval-system-100vu-10k-itr.png)

The system-only run isolates Redis cache-hit + Postgres-fallback latency from external network noise, which is the number that actually reflects the cache-aside design (see section 4).

## 6. Getting started

**Prerequisites:** Go 1.25+, a Postgres database (this project uses Supabase), `sqlc`, and optionally `air` for hot reload and Redis for caching.

```bash
# 1. Clone and install Go deps
git clone <repo-url>
cd Lynkr
go mod download

# 2. Set up your database
#    - Create a project in Supabase (or use any Postgres instance)
#    - Open the SQL Editor and run the contents of sql/schema.sql
#      (enable Row Level Security with no policies if using Supabase,
#      since this app connects directly and doesn't need PostgREST/anon access)

# 3. Configure environment
cp .env.example .env   # if present, otherwise create .env manually:
#   DATABASE_URL=postgres://user:password@host:port/dbname
#   PORT=8080
#   REDIS_URL=redis://user:password@host:port   # optional, caching disabled if unset

# 4. Generate the DB layer (only needed if internal/database/ is missing or sql/*.sql changed)
sqlc generate

# 5. Run the server
go run ./cmd/server
# or, for hot reload on file changes:
air

# 6. Try it out
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"target_url": "https://example.com", "title": "Example"}'

curl -L http://localhost:8080/<code>                 # redirects to the target URL
curl 'http://localhost:8080/api/links?page=1'         # lists links, newest activity first
curl -X DELETE http://localhost:8080/api/links/<code>  # deletes a link
```
