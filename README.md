# Lynkr

## 1. What this app is

Lynkr is a URL shortener built to learn Go fundamentals and system design by building something real, rather than following a toy tutorial. It started with a React frontend calling a JSON API, and is now being reorganized around a plain Go backend (chi router + PostgreSQL via Supabase), with server-rendered Go templates replacing the separate frontend.

Core features:
- Shorten a long URL into a short, random alias (`go-nanoid`-generated code)
- Redirect `/{shortKey}` to the original URL
- Track click counts per short URL (via a background stats upsert on redirect)
- List all shortened URLs and their stats

## 2. Repo structure

```
.
├── cmd/
│   └── server/            # the `main` package — the actual binary/entrypoint
│       ├── main.go        # wiring: env, db pool, Config struct, http.Server
│       ├── env.go          # Env: loads/validates DATABASE_URL, PORT from .env
│       ├── handlers.go     # Data + Handlers: DB deps + HTTP handler methods (ShortenURL, Redirect, ListURLs)
│       ├── routes.go       # Routes: chi router setup, middleware, route table
│       ├── helpers.go      # Codex: shared JSON read/write/error helpers
│       └── templates/      # Go html/template files (server-rendered UI)
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
├── scripts/test.js         # load testing script
├── go.mod / go.sum
└── README.md
```

The rule of thumb: `cmd/` holds only `main` packages (binaries); `internal/` holds shared logic (like the DB layer) that isn't allowed to be imported outside this module.

Within `cmd/server`, the app is wired as a small dependency graph, all hanging off a top-level `Config` struct in `main.go`:

```
Config
├── Env      — loaded env vars (DATABASE_URL, PORT)
├── Data     — *database.Queries + *pgxpool.Pool
├── Handlers — HTTP handler methods, depends on Data + Codex
├── Routes   — chi router, depends on Handlers
└── Codex    — JSON read/write/error helpers
```

## 3. sqlc — benefits & commands

**What it does:** sqlc reads plain SQL (`sql/schema.sql` for table shape, `sql/queries.sql` for queries annotated with `-- name: X :one/:many/:exec`) and generates type-safe Go code — no ORM, no reflection, no ORM-generated queries. You write real SQL; sqlc turns it into Go functions and structs.

**Why it's used here:**
- **Type safety** — `Queries.CreateURL(ctx, CreateURLParams{...})` is a real Go function with real Go types; a typo in a column name fails at `sqlc generate` or compile time, not at runtime in production.
- **No magic** — the SQL you write is the SQL that runs. Easy to reason about performance, indexes, and query plans since there's no query builder translating your intent.
- **Structs match the DB** — `models.go` is regenerated straight from `schema.sql`, so Go structs and table columns can't silently drift apart.
- **Good fit for learning Go/system design** — forces you to actually write and understand SQL, while still getting compile-time safety on the Go side.

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

## 4. Getting started

**Prerequisites:** Go 1.25+, a Postgres database (this project uses Supabase), `sqlc`, and optionally `air` for hot reload.

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

# 4. Generate the DB layer (only needed if internal/database/ is missing or sql/*.sql changed)
sqlc generate

# 5. Run the server
go run ./cmd/server
# or, for hot reload on file changes:
air

# 6. Try it out
curl -X POST http://localhost:8080/shorten \
  -H "Content-Type: application/json" \
  -d '{"longURL": "https://example.com"}'

curl -L http://localhost:8080/<shortKey>   # redirects to the original URL
curl http://localhost:8080/urls            # lists all shortened URLs
```
