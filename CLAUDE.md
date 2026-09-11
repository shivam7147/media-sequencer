# CLAUDE.md — Multi-Window Media Sequencer

Read this and [PLAN.md](PLAN.md) before starting any prompt in this project. PLAN.md has the
full reasoning; this file is the fast-reference summary plus current status.

Deadline: **12 AM Saturday 13 September 2026**.

## The one idea

What a window shows is a pure function of the current time — `currentItem(window, now)`.
Nothing stores "where we are." A reload, a sync, a second browser window: all just re-evaluate
the same function against the same clock. Never move off this model.

## Locked decisions

| Area | Choice |
|---|---|
| Backend | Go 1.25 + `chi` (module `github.com/shivam7147/media-sequencer/backend`) |
| Storage | Postgres (Render free instance), `jackc/pgx/v5` stdlib adapter |
| Live updates | SSE (`text/event-stream`), not WebSockets |
| Frontend | React via Vite, plain JavaScript (no TypeScript) |
| Deployment | Render Static Site (frontend) + Render Web Service/Docker (backend) + Render Postgres |
| Media | Bundled assets in `frontend/public/media`, seeded as M1–M6 |
| Repo | Single repo, `backend/` + `frontend/` |
| Local dev DB | Render Postgres external connection string via `backend/.env` — no local Postgres install |

## Schedule maths

Per window: `anchor` (global epoch timestamp), `items` = ordered `[(media, duration)]`,
`P = Σ durations`, `C` = cycle length in seconds (default **18000** = 5h, runtime-configurable).

```
elapsed = (t − anchor) mod C        // position within the cycle
offset  = elapsed mod P             // position within the looping playlist
walk items accumulating durations until offset falls inside item k
→ item k, with (start_of_k + d_k − offset) seconds remaining
```

Clamp: if an item would run past the cycle boundary, cut it there so every window restarts
together.

**Edge cases (must hold in both Go and JS implementations):**

| Case | Behaviour |
|---|---|
| Empty playlist | Show blank, never crash |
| `P > C` | Playlist truncated at the cycle boundary; the tail never plays |
| `P` doesn't divide `C` | Final repetition is cut short at the boundary — intended, keeps cycles aligned |
| Blank in playlist | Ordinary item with its own duration; never appears unless configured |
| Playlist changed mid-cycle | Recompute immediately from the same anchor; windows may jump — expected |
| `now` before `anchor` | Must not crash or go negative — clamp/mod correctly |

**Sync overlay** (applied after the schedule above):

```
if a sync event exists with start_at ≤ now < start_at + duration:
    every window shows sync.media, with (start_at + duration − now) remaining
else:
    each window shows its own computed item
```

`internal/schedule` (Go) and `frontend/src/schedule.js` must implement identical logic —
that agreement is what makes refresh-proofing and multi-window sync work at all.

## Data model

```sql
media          (id, label, kind ['image'|'video'|'blank'], url, default_duration_seconds)
windows        (id, name, position)
window_items   (id, window_id, media_id, position, duration_seconds)
sync_events    (id, media_id, start_at, duration_seconds, created_at)
settings       (key, value)        -- 'cycle_seconds', 'anchor'
```

Seed: 4 windows, 6 media (M1–M6: four colour cards, one short video, one blank), each window
gets a different subset/order so they're visibly out of phase until sync is triggered.

## API contract

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | `{"status":"ok"}` |
| `GET` | `/api/time` | `{"server_time":"…RFC3339Nano"}` — clock sync |
| `GET` | `/api/state` | Server time, anchor, cycle seconds, all windows+items, media library, active sync |
| `POST` | `/api/windows/{id}/items` | `{media_id, duration_seconds?}` → append to playlist |
| `DELETE` | `/api/windows/{id}/items/{itemId}` | Remove an item |
| `POST` | `/api/media` | `{label, kind, url, default_duration_seconds}` → add to library |
| `POST` | `/api/sync` | `{media_id, duration_seconds}` → start a sync now |
| `PUT` | `/api/settings/cycle` | `{cycle_seconds}` → demo control |
| `GET` | `/api/events` | SSE stream: `state_changed`, `sync` |

Errors: `{"error":"message"}`. CORS allows only origins listed in `ALLOWED_ORIGIN` (never `*`,
never an origin that isn't an exact match); SSE responses go through the same CORS middleware
as everything else.

## Environment variables (backend)

| Var | Purpose | Local dev default |
|---|---|---|
| `PORT` | HTTP listen port | `8080` |
| `DATABASE_URL` | Render Postgres external connection string | (none — set in `backend/.env`) |
| `ALLOWED_ORIGIN` | Comma-separated list of exact origins allowed to call the API (CORS) | `http://localhost:5173` |

`backend/.env` (gitignored) is loaded via `godotenv` if present; `backend/.env.example`
documents the shape. Production (Render) sets real env vars — no `.env` file there.

## Traps to not re-learn the hard way

1. Never use raw `Date.now()` client-side — always go through the measured server-clock offset.
2. Never use `setInterval` for playback — schedule one timer for exactly the remaining seconds
   of the current item, then recompute.
3. Video needs `muted`, `playsInline`, `autoplay` or autoplay silently fails.
4. SSE needs a heartbeat comment every ~20s or Render's proxy closes the idle stream; client
   must reconnect on drop.
5. `ALLOWED_ORIGIN` must be explicit, not `*` — it's now a comma-separated list matched exactly
   per-request and echoed back only on a match, so both the deployed frontend and
   `http://localhost:5173` can be listed at once; this applies to SSE too.
6. Static host needs a `/*` → `/index.html` rewrite or `/window/:id` 404s on refresh.
7. Seed must be idempotent (`CREATE TABLE IF NOT EXISTS` + guarded inserts) or every restart
   duplicates rows.
8. Ship the real default: `cycle_seconds = 18000`. The demo control changes it at runtime —
   don't quietly ship 60s as the default.

## Status checklist

Work through these in order; each has its own verify step in PLAN.md § Execution pipeline.
Update the checkbox here immediately after a step is verified, before starting the next.

- [x] **Step 1** — Repo, backend skeleton, `/health`
- [x] **Step 2** — Postgres: provision, connect, migrate, seed
- [x] **Step 3** — Schedule engine + exhaustive tests (`internal/schedule`)
- [x] **Step 4** — Store + read API (`GET /api/state`, `GET /api/time`)
- [x] **Step 5** — Write API (add/delete item, add media, sync, cycle length)
- [x] **Step 6** — SSE (`GET /api/events`, broadcast on writes) + CORS
- [x] **Step 7** — Dockerfile (backend not yet deployed to Render — that's the user's next action)
- [x] **Step 8** — React shell + clock offset + Wall rendering
- [x] **Step 9** — Playback (`MediaFrame`), single-window route, refresh-proof timers
  (⚠ `frontend/public/media/m5.mp4` still doesn't exist — see note below, deferred by the user)
- [x] **Step 10** — SSE wiring + controls panel (media, sync, cycle length)
- [ ] **Step 11** — Deploy frontend + CORS wired to deployed backend
- [ ] **Step 12** — README, `go test`/`go vet`/`gofmt` clean, repo public, submit

### Step 1 — done

- `git init` at repo root.
- `backend/` scaffolded per PLAN.md layout: `cmd/server/`, `internal/{config,database,models,
  schedule,store,sse,middleware,handlers}` (empty subpackages hold `.gitkeep` until their step).
- `frontend/` directory skeleton only (`src/{components,pages}`, `public/media/`) — no app yet,
  that's Step 8.
- Go module: `github.com/shivam7147/media-sequencer/backend`, `go 1.25.0`, deps `go-chi/chi/v5`
  and `joho/godotenv`.
- `internal/config`: reads `PORT` / `DATABASE_URL` / `ALLOWED_ORIGIN` from env with defaults.
- `cmd/server/main.go`: chi router with `middleware.Logger` + `middleware.Recoverer`,
  `GET /health` → `{"status":"ok"}`, graceful shutdown via `signal.NotifyContext` +
  `srv.Shutdown` with a 10s timeout.
- `.gitignore` (root), `backend/.env.example`.
- Verified: `go build ./...`, `go vet ./...`, `gofmt -l .` all clean; `go run ./cmd/server` then
  `curl localhost:8080/health` → `{"status":"ok"}`; SIGTERM stops it and the port frees up.
  (Note: SIGTERM delivery through Git Bash on native Windows doesn't reliably trigger Go's
  signal handler the way it does on Linux — the code path is correct and will be exercised for
  real under Docker/Render, which is Linux. Not a concern, just don't re-debug it here.)

### Step 2 — done

- Postgres already provisioned on Render (`backend/.env` has a live external `DATABASE_URL`,
  gitignored) — provisioning happened before this step started.
- `internal/database/database.go`: `NewPool` opens a `pgxpool.Pool` (MaxConns 10, MinConns 1,
  30m max lifetime, 5m max idle, 1m health check), pings with a 10s timeout so a bad connection
  string or unreachable host fails at boot with a clear error, not on the first request.
  `Migrate` runs one `CREATE TABLE IF NOT EXISTS` block for `media`, `windows`, `window_items`,
  `sync_events`, `settings`, plus `idx_window_items_window_position` — every column that holds a
  time is `TIMESTAMPTZ`, `window_items` has FKs to both `windows` (`ON DELETE CASCADE`) and
  `media` (`ON DELETE RESTRICT`).
- `internal/database/seed.go`: `Seed` runs inside one transaction, checks `COUNT(*) FROM media`
  first — if non-zero it commits a no-op and returns `seeded=false`; otherwise it inserts all 6
  media, all 4 windows with their items, and the two `settings` rows (`cycle_seconds=18000`,
  `anchor=2026-01-01T00:00:00Z` — a fixed constant, never `time.Now()`, and never overwritten on
  later boots via `ON CONFLICT (key) DO NOTHING`). This makes restart-without-duplication
  structural, not just convention.
- `cmd/server/main.go`: boots the pool → migrate → seed → log one line
  (`"database: tables ready, seed data inserted"` or `"...already present"`) before starting the
  HTTP server; `pool.Close()` is deferred alongside the existing graceful shutdown.
- Seed data actually inserted (see table below) — subset/order per window is deliberate so the
  wall is visibly out of phase pre-sync:

  | Window | Items (label, duration) |
  |---|---|
  | Window 1 | M1(8s), M2(10s), M3(7s) |
  | Window 2 | M3(7s), M4(9s), M5(15s) |
  | Window 3 | M5(15s), M1(8s), M6(5s), M2(10s) |
  | Window 4 | M6(5s), M4(9s), M2(10s), M3(7s) |

  Media: M1–M4 images (`/media/m1.svg`…`/media/m4.svg`, real 1280×720 SVG cards now in
  `frontend/public/media/`, strong distinct colours + large centred label), M5 video
  (`/media/m5.mp4` — URL seeded, file itself not created yet, out of scope for this step), M6
  blank (empty `url`).
- Verified against the **live** Render Postgres: first boot logged `seed data inserted` and
  actually wrote 6 media / 4 windows / 14 window_items / 2 settings rows; a second boot logged
  `seed data already present` with zero new rows (checked directly via a throwaway query
  script — counts unchanged, no duplicates). `go build`, `go vet`, `gofmt -l .` all clean.

### Step 3 — done

- `internal/schedule/schedule.go`: pure functions, no database or HTTP imports — portable to
  `frontend/src/schedule.js` near line-for-line when Step 8 needs it.
  - `Item{MediaID, DurationSeconds}`, `SyncEvent{MediaID, StartAt, DurationSeconds}`,
    `Resolved{MediaID, Remaining, IsSync}` as specified.
  - `floorMod(a, n time.Duration) time.Duration` — always returns `[0, n)`, unlike Go's `%`
    which goes negative for a negative dividend. Used for both the cycle wrap
    (`now - anchor`) and the playlist wrap (`elapsed mod P`), so a `now` before `anchor` (clock
    skew, wrong client clock, future anchor) still produces a valid non-negative position
    instead of garbage.
  - Non-positive-duration items are excluded from the playlist-duration sum and skipped in the
    walk — necessary for negative durations specifically (they'd shrink the running total and
    throw off every later comparison); zero-duration items are harmless either way but skipped
    for clarity. The single linear pass over `items` is bounded by slice length regardless, so
    there's no possible infinite loop here by construction.
  - `minRemaining = time.Second` floors every `Remaining` — landing exactly on a boundary, or
    the cycle-end clamp, can otherwise produce zero or a sub-second value that would make a
    client schedule a zero-delay timer and spin.
  - `CurrentItem` returns `(Resolved, bool)` — `ok=false` means "nothing valid to show" (empty
    playlist or every item invalid); `Resolved.MediaID` is then the zero value and
    `Resolved.Remaining` is clamped time-to-cycle-end, so the caller knows when it's worth
    checking again.
  - Playlist-longer-than-cycle truncation, and the final-repetition-cut-short-when-P-doesn't-
    divide-C behaviour, both fall out of one shared computation
    (`remaining = min(item's natural remaining, time to cycle end)`) — no special-casing needed
    per scenario.
  - `Resolve` layers the sync overlay on top of `CurrentItem` using plain `time.Time` comparison
    (`start_at <= now < start_at+duration`, half-open) since a sync is a one-shot absolute
    interval, not cyclic — the base schedule is never touched, so when the sync ends every
    window is exactly where its own schedule would already have put it.
- `schedule_test.go`: table-driven, all cases from the brief covered and asserting both
  `MediaID` and `Remaining` (plus `ok`/`IsSync` where relevant): mid-item; exact start instant;
  exact end instant; wrap at end of playlist; wrap at end of cycle (multiple cycles elapsed);
  empty playlist; single item; one item longer than the whole cycle; playlist not dividing the
  cycle (with a case where truncation actually bites, not just coincides with the item's own
  end); now == anchor; now before anchor; a zero-duration item mixed in; the `minRemaining`
  clamp on a genuine sub-second case; sync active; sync just expired (boundary instant); sync in
  the future; sync with a non-positive duration (guard added alongside the item-duration guard).
- Verified: `go test ./internal/schedule/... -v` — all 18 subtests pass. `go build ./...`,
  `go vet ./...`, `gofmt -l .` all clean.

### Step 4 — done

- `internal/models`: `Media`, `WindowItem`, `Window` (with `Items []WindowItem`), `SyncEvent`,
  `Settings` — plain structs mirroring the schema, no DB tags needed since `store` scans
  positionally.
- Extended `schedule.Resolved` with a `Blank bool` field (touches Step 3's code — noted here
  since that step's CLAUDE.md entry described the struct without it). Set `true` at all three
  "nothing valid to show" return points in `CurrentItem`, left `false` (zero value) everywhere
  else, including when `Resolve` returns an active sync. This keeps "is there really nothing to
  show" as a single source of truth inside the schedule package rather than handlers
  re-deriving it from the `(Resolved, bool)` return or guessing from `MediaID == 0`. Existing
  `schedule_test.go` cases don't assert on `Blank` and all still pass unchanged.
- `internal/store/store.go`: `Store` wraps `*pgxpool.Pool`. One method per thing, all
  `context.Context`-first, all returning `internal/models` types — no SQL leaks outside this
  package.
  - `ListMedia` — ordered by id.
  - `ListWindows` — two queries total (windows, then all window_items ordered by
    `window_id, position`), grouped in Go — not N+1.
  - `GetSettings` — reads both `settings` rows in one query, parses `anchor` via
    `time.RFC3339Nano` and `cycle_seconds` via `strconv.Atoi`, errors clearly if either key or
    parse is missing/bad.
  - `ActiveSync(ctx, now)` — `start_at <= now AND start_at + (duration_seconds * interval '1
    second') > now`, ordered `start_at DESC LIMIT 1`; returns `(nil, nil)` on `pgx.ErrNoRows`
    rather than an error, since "no active sync" is the normal case.
- `internal/handlers`: `Handlers{Store}`, `New(*store.Store)`.
  - `Time` — no DB call, just `time.Now().UTC().Format(RFC3339Nano)`; kept deliberately cheap
    since the client samples it repeatedly for clock-offset estimation.
  - `State` — calls all four store methods, converts to JSON types, and resolves every window
    server-side via `schedule.Resolve` (same `activeSync` overlay object reused across all
    windows, matching "every window shows sync.media"). `resolved.blank` is copied straight from
    `schedule.Resolved.Blank` — never inferred from `media_id == 0` — and when blank, `media_id`
    is omitted (zero value) and `kind` is reported as `"blank"`. A comment on the handler
    explains why resolving server-side matters: `curl /api/state` alone shows exactly what every
    window should be displaying at that instant, no browser or client maths required.
  - Errors use round 1's `{"error":"message"}` shape via a small `writeJSON`/`writeError` helper
    in `json.go`; store failures are logged server-side with detail and reported to the client
    generically (500, no SQL leaked).
- `cmd/server/main.go`: wires `store.New(pool)` into `handlers.New(...)`, registers
  `GET /api/time` and `GET /api/state` alongside the existing `/health`.
- Verified against the **live** Render Postgres: `curl /api/time` and `curl /api/state` both
  return correct, well-formed JSON; the four seeded windows show different `resolved` items
  (confirming they're genuinely out of phase pre-sync) with consistent fractional
  `remaining_seconds`. Inserted a real `sync_events` row directly via a throwaway script and
  re-curled `/api/state`: `active_sync` populated, all four windows flipped to
  `is_sync:true` with the same `media_id` and matching countdown — confirmed the SQL interval
  query and the overlay both work end-to-end, then deleted the test rows to leave the database
  clean. `go build`, `go vet`, `gofmt -l .` all clean.

### Step 5 — done

- `internal/store/writes.go`: `ErrNotFound` sentinel; handlers turn it into 404, anything else
  into 500 with the detail logged server-side only.
  - `AppendWindowItem(ctx, windowID, mediaID, durationSeconds *int)` — one transaction: check
    window exists, fetch media's `default_duration_seconds` (also doubles as the media-exists
    check), compute `position = MAX(position)+1` for that window, insert. `durationSeconds` nil
    means "use the media's default"; the transaction is what stops two concurrent appends to the
    same window from racing on position.
  - `DeleteWindowItem(ctx, windowID, itemID)` — single `DELETE ... WHERE id = $1 AND window_id =
    $2`; zero rows affected means not-found (covers both "doesn't exist" and "belongs to a
    different window" in one check). Comment explains the deliberate choice not to renumber
    remaining positions — ordering only depends on relative order, not contiguity, so gaps are
    harmless and renumbering would just be extra writes for no behavioural benefit.
  - `CreateMedia`, `CreateSync`, `SetCycleSeconds` — straightforward inserts/upserts.
    `SetCycleSeconds` upserts via `ON CONFLICT (key) DO UPDATE`, never touches `anchor`.
  - `CreateSync` comment: a newer sync always outranks whatever was active, because `ActiveSync`
    (Step 4) already orders by `start_at DESC` — no locking, no "already syncing" error, this is
    the deliberate answer to two people triggering sync at nearly the same time.
- `internal/handlers`: added `items.go` (`AddWindowItem`, `DeleteWindowItem`), `media.go`
  (`CreateMedia`), `sync.go` (`CreateSync`), `settings.go` (`SetCycleSeconds`). Each does
  request-shape validation (required fields, `kind` enum, `duration_seconds`/
  `default_duration_seconds` > 0, `url` empty only for `kind: "blank"`) before calling the store;
  existence checks (window/media/item) live in the store since they need the database.
  `POST /api/sync` always uses `time.Now().UTC()` server-side for `start_at`, never a
  client-supplied value — commented why (clock skew makes a client-chosen "now" meaningless for
  something every window must agree on).
  - `PUT /api/settings/cycle` clamps to `[10, 86400]` seconds — commented that the shipped
    default is 18000 (5h) and this endpoint exists purely so a reviewer can drop it to ~60s and
    watch the cycle restart in under a minute.
  - `Handlers.broadcast(event string)` in `handlers.go` — the single marked no-op every write
    handler already calls (`"state_changed"` for item/media/settings writes, `"sync"` for
    `POST /api/sync`). Step 6 only has to give this one function a body.
- `cmd/server/main.go`: registered all five write routes alongside the existing read/health
  routes.
- Verified against the **live** Render Postgres with real `curl` calls for every case in the
  brief: add item (fallback duration and explicit duration), 404 on unknown window/media, 400 on
  non-positive duration; delete item (204), 404 on unknown item and on an item requested through
  the wrong window, confirmed the deleted items are actually gone and the remaining ones kept
  their original positions (gap not renumbered); create media (201), 400 on bad `kind`, 400 on
  empty `url` with a non-blank kind, 201 for blank+empty-url, 400 on missing `label` and on
  non-positive `default_duration_seconds`; create sync (201), confirmed `active_sync` populates
  and all 4 windows flip `is_sync:true` together, 404 on unknown media, 400 on non-positive
  duration, and confirmed a second sync supersedes the first (`active_sync` moved to the new
  `media_id`); set cycle (200) confirmed reflected in `/api/state`, 400 below 10 and above
  86400. Cleaned up every row the tests created (test media, test sync events) and restored
  `cycle_seconds` to 18000 afterward — live DB verified back to exactly the Step 2 seed state.
  `go build`, `go vet`, `gofmt -l .`, `go test ./...` all clean.

### Step 6 — done

- `internal/sse/broker.go`: `Broker{subscribers map[chan string]struct{}}`, mutex-guarded.
  `Subscribe()` returns a receive-only channel (buffered, size 4) plus an `unsubscribe` closure
  that deletes-and-closes exactly once. `Broadcast(event)` sends to every subscriber with
  `select { case ch <- event: default: }` — comment explains why non-blocking is load-bearing,
  not just a nicety: every write handler calls `Broadcast` synchronously after committing, so a
  blocking send to one slow/dead subscriber would stall every future write for every client, not
  just that one connection. Dropping an event is safe because an event carries no data — it's
  purely "go refetch /api/state" — so a client that misses one is still correct the moment it
  refetches.
- `internal/handlers/events.go`: `Events` requires `http.Flusher` (500 if the response writer
  doesn't support it), sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive`, `X-Accel-Buffering: no` (stops Render's proxy from buffering the
  stream instead of forwarding it live). Sends `event: connected` immediately post-headers so
  the client can tell "live, waiting" apart from "still connecting". A 20s `time.Ticker` sends
  `: ping\n\n` (an SSE comment line, invisible to `EventSource` listeners) — commented why:
  without it Render's proxy would close the connection as idle and the page would silently stop
  updating with no error. Returns (and the deferred `unsubscribe` runs) when
  `r.Context().Done()` fires. Event payloads are always `data: {}` — comment explains clients
  refetch `/api/state` rather than trust a pushed value, so there's no ordering or staleness to
  reason about on this stream at all.
- `Handlers` gained a `Broker *sse.Broker` field (`New(store, broker)`); `broadcast()` from
  Step 5 now calls `h.Broker.Broadcast(event)` instead of being a no-op — the one-function change
  that step's comment promised.
- `internal/middleware/cors.go`: `CORS(allowedOrigin)` sets `Access-Control-Allow-Origin` to
  exactly the configured origin (never reflects the request's own `Origin`, never `*`),
  `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`,
  `Access-Control-Allow-Headers: Content-Type`, and short-circuits `OPTIONS` with a 204. Applied
  via `r.Use(...)` ahead of every route in `main.go`, so it covers `/api/events` the same as
  every other endpoint.
- `cmd/server/main.go`: chi's own middleware package is now aliased `chimiddleware` to make room
  for this project's `internal/middleware`; registered `sse.NewBroker()`, passed into
  `handlers.New`, and added `GET /api/events`.
- Verified against the **live** Render Postgres + a real running server: CORS headers correct on
  both a plain `GET /api/state` and an `OPTIONS` preflight for `POST /api/media`; confirmed the
  allowed origin is the fixed configured value even when the request sends a different `Origin`
  (not reflected). Opened `GET /api/events` with `curl -N` and confirmed, on the same live
  connection: the exact SSE headers, an immediate `event: connected`, then triggered
  `PUT /api/settings/cycle` and `POST /api/sync` from a second terminal and watched
  `event: state_changed` and `event: sync` arrive on the open stream in real time; waited past
  20s and confirmed two `: ping` heartbeat lines; killed the curl client and confirmed the server
  logged a clean disconnect (`GET /api/events ... 200 ... 1m4s`, no error) rather than hanging or
  erroring. Cleaned up the test sync event afterward — live DB back to the seed state.
  `go build`, `go vet`, `gofmt -l .`, `go test ./...` all clean.

### Step 7 — done

- **CORS change (requested alongside this step):** `ALLOWED_ORIGIN` is now a comma-separated
  list. `internal/middleware/cors.go` splits it into a set, matches the request's `Origin`
  header exactly, and echoes back only that exact value — never `*`, never an unmatched origin
  (no `Access-Control-Allow-Origin` header at all in that case, which is what makes the browser
  block it). Added `Vary: Origin` since the response now depends on the request's `Origin`.
  `config.Config.AllowedOrigins` (renamed from `AllowedOrigin`) defaults to
  `http://localhost:5173` instead of the old `*` fallback — under the new match-and-echo design
  a literal `"*"` default would never match a real browser `Origin` header anyway, so it's
  changed to a concrete origin that actually works for local dev out of the box (fail-closed if
  truly unconfigured, not silently-permissive). `backend/.env.example` updated to show a
  two-origin example (localhost + the eventual deployed frontend URL).
  - Verified live: two-origin `ALLOWED_ORIGIN`, confirmed both listed origins get echoed back
    correctly (with `Vary: Origin`), an unmatched origin gets no `Access-Control-Allow-Origin`
    header, and a request with no `Origin` header also gets none — then restored the real
    `.env` to its original single-origin value.
- `backend/Dockerfile`: multi-stage, assumes the build context is `backend/` (not the repo
  root) — every `COPY` path is relative to that.
  - Stage 1 `golang:1.25-alpine`: `go.mod`/`go.sum` copied and `go mod download` run as their
    own layer before the rest of the source, so dependency downloads are cached across builds
    that only touch source. `CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w"` — comment notes
    pgx is pure Go (speaks the Postgres wire protocol itself, no libpq/C dependency), so no C
    toolchain is needed and the binary is fully static.
  - Stage 2 `alpine:3.20`: `ca-certificates` installed (commented why: the DB connection uses
    `sslmode=require`, so TLS verification needs root certs or every connection fails), a
    non-root `app` user, binary copied in, `EXPOSE 8080`, `HEALTHCHECK` via `wget` against
    `/health`, `CMD ["/app/server"]`. No data directory — all state is in Postgres, the
    container itself is disposable. Final image: **29.1MB**.
- `backend/.dockerignore`: excludes `.env`/`.env.*` (keeping `.env.example`), `*.md`, `.git`,
  editor/OS cruft, and Go build artifacts that aren't needed to compile.
- Verified with a real local Docker build (Docker Desktop wasn't running — started it and
  waited for the daemon before proceeding): `docker build` succeeds cleanly in two stages; ran
  the built image against the **live** Render Postgres via `DATABASE_URL` and confirmed in the
  container logs `database: tables ready, seed data already present` and `listening on :8080`;
  `curl /health` through the container returns `{"status":"ok"}` with the CORS headers already
  applied; confirmed the process runs as `uid=100(app)`, not root; confirmed Docker's own
  `HEALTHCHECK` reports `"Status":"healthy"` after its first successful probe. Cleaned up the
  test container and image afterward.
- **Not done as part of this step** (explicitly deferred to the user, per the brief): actually
  provisioning the Render Web Service, setting `DATABASE_URL`/`ALLOWED_ORIGIN` there, and
  deploying. The Dockerfile is ready for that; nothing here has touched Render.

### Step 8 — done

- **Scaffolding:** `npm create vite@latest` refused a path with a colon/backslashes when passed
  as an absolute Windows temp path (silently created a mis-named relative directory instead —
  cleaned up and not left behind); worked once run from inside the temp directory with a plain
  relative name. Scaffolded into `$env:TEMP\vite-scaffold-mediaseq` (React template, JS not TS),
  then merged the generated `package.json`, `vite.config.js`, `index.html`, `.gitignore`,
  `.oxlintrc.json`, `src/main.jsx` into `frontend/` — `public/media/m1.svg`…`m4.svg` were never
  touched by the scaffold step and are still exactly the Step 2 seed files. Discarded the
  scaffold's own `App.jsx`/`App.css`/`assets/` (React/Vite demo content) and its landing-page
  `index.css` in favor of this project's own. Current toolchain from the scaffold: Vite 8,
  React 19, `oxlint` (not eslint) for linting.
- `src/api.js` — thin `fetch` wrapper (`request(path, options)`) reading `VITE_API_BASE` from
  `import.meta.env`; throws an `Error` carrying the backend's own `{"error": "..."}` message on
  a non-2xx response, falling back to the HTTP status line only if the body isn't JSON.
  `getState()`/`getTime()` are used starting this step; `addWindowItem`, `deleteWindowItem`,
  `createMedia`, `createSync`, `setCycleSeconds` are implemented as real (not placeholder) thin
  wrappers matching the API contract exactly, but nothing calls them yet — Step 10 wires them
  into the controls panel.
- `src/clock.js` — `syncClock()` samples `GET /api/time` 5 times, computes
  `offset = serverTime - (localBefore + rtt/2)` per sample, and keeps the **lowest-RTT** sample
  rather than averaging — commented why: a slow round trip could have been slow outbound,
  inbound, or queued in between, so the fastest sample is the one where "the response landed
  halfway through the round trip" is closest to true. `serverNow()` returns
  `Date.now() + offset`; `startClockSync()` runs an initial sync then re-samples every 5 minutes,
  returning a cleanup function. Comment on the module explains why this exists at all: every
  window derives what it shows from the clock, so a browser with a wrong clock would otherwise
  confidently show the wrong media.
- `src/schedule.js` — direct port of `internal/schedule/schedule.go`, explicitly commented as
  such (Go is the reference; fix this file to match Go, never the reverse). Same `floorMod`
  (JS `%` has the identical negative-operand bug as Go's), same non-positive-duration guard in
  both the sum and the walk, same `MIN_REMAINING_MS = 1000` clamp, same
  `min(item's natural remaining, time to cycle end)` line, same half-open sync-overlay interval
  check. The one deliberate difference: Go's `time.Duration` (nanoseconds) becomes plain
  milliseconds throughout, since that's the native unit of `Date.now()`/`serverNow()` — every
  formula has the same shape, just in that unit. Exports `floorMod`, `currentItem` (returns
  `{resolved, ok}`, mirroring Go's `(Resolved, bool)`), and `resolve`.
  - **Verified independently of the UI**: wrote a throwaway Node script re-running the exact
    same 17 cases from `schedule_test.go` (mid-item, exact start/end instants, wrap at end of
    playlist, wrap at end of cycle across multiple cycles, empty playlist, single item, item
    longer than the whole cycle, `P` not dividing `C`, now == anchor, now before anchor,
    zero-duration item mixed in, the `MIN_REMAINING_MS` clamp on a genuine sub-second case, sync
    active, sync just expired, sync in the future, `floorMod` on a negative operand) against
    `schedule.js` directly — all 17 passed with identical expected values to the Go suite, then
    deleted the script.
- `src/components/WindowTile.jsx` — renders the window name, a `SYNC` badge when
  `resolved.isSync`, remaining seconds, and the media itself: a real `<img>` for `kind: "image"`,
  a labelled placeholder for `video` and for `blank`/`resolved.blank` (proper `<video>` and blank
  handling is Step 9's `MediaFrame`). Comment distinguishes `resolved.blank` ("nothing valid to
  show" from the schedule) from a configured blank media item (real id/label, just empty `url`).
- `src/pages/Wall.jsx` — fetches `/api/state` once on mount; converts `active_sync` and each
  window's `items` from the API's snake_case shape into `schedule.js`'s camelCase shape once,
  memoized. **One shared `setInterval(..., 250)`** re-reads `serverNow()` and triggers a
  recompute of every window on each tick, rather than a per-window timer — comment explains why
  that's sufficient and simpler: because nothing about what a window shows is stored (it's
  recomputed fresh from the anchor and the clock every tick), a coarse tick is self-correcting
  and there is no accumulating drift to manage, so one timer driving every window is simpler to
  reason about than N independent ones. Renders a responsive `auto-fit` CSS grid of
  `WindowTile`s.
- `frontend/.env` / `.env.example`: `VITE_API_BASE=http://localhost:8080`. Confirmed via
  `git check-ignore -v frontend/.env` that the root `.gitignore`'s bare `.env` pattern already
  covers it (matches at any depth), and `!*.env.example` correctly keeps the example file
  tracked — no frontend-specific `.gitignore` entry was needed for this.
- Verified against a **live** local backend (pointed at the real Render Postgres) and the Vite
  dev server together: `npm run lint` (oxlint) and `npm run build` both clean (one React
  Compiler memoization warning on first build, fixed by correcting a `useMemo` dependency array
  to `[state]`, then clean). Started the Go backend and `vite dev` side by side; confirmed the
  backend's CORS middleware correctly allows `Origin: http://localhost:5173`; confirmed all four
  seeded SVGs (`/media/m1.svg`…`m4.svg`) are served by the Vite dev server itself (bundled
  frontend assets, not backend-served); confirmed every new source module
  (`main.jsx`/`App.jsx`/`Wall.jsx`/`WindowTile.jsx`/`api.js`/`clock.js`/`schedule.js`) transforms
  through Vite's dev server with no error. **Not verified**: actual in-browser rendering — no
  browser automation tool is available in this environment, so the grid's visual appearance and
  live countdown/looping behavior (the brief's own verify step: "windows show different media
  and change on their own; set cycle to 60s and watch them loop") still needs a real browser,
  which is what the user will do next. Stopped both dev servers and freed their ports afterward.

  Everything downstream of `/api/state` was checked as rigorously as possible without a browser:
  the schedule maths (the part most likely to be silently wrong) is verified byte-for-byte
  against the Go reference's own test cases; the network layer (CORS, static assets, API JSON)
  is verified live; only the final "does React actually paint this correctly" step is left for
  visual confirmation.

### Step 9 — done, with one open item

- **`frontend/public/media/m5.mp4` is still missing.** `ffmpeg` isn't installed (checked both
  Git Bash's and native Windows' `PATH` — not found either way). Asked the user how to proceed;
  they chose to skip video generation for now rather than install ffmpeg or supply a file. The
  seed still points `M5` at `/media/m5.mp4` (Step 2), so until that file exists, `M5` will hit
  `MediaFrame`'s video `onError` fallback and render as `"M5 (missing)"` instead of playing —
  confirmed this actually happens (see verify notes below), so it fails visibly, not silently.
  Revisit when ffmpeg is available or a clip is supplied — no code change needed either way, just
  drop the file in place.
- `src/schedule.js` gained one JS-only field beyond the Go port: `resolved.elapsedMs` — how far
  into its current slot the resolved item already is. Computed as `offset - acc` at the point an
  item matches in `currentItem`'s walk (always ≥ 0 by the loop's own invariant, and unaffected by
  the cycle-boundary clamp, since that only shortens what's left, not what's already played), and
  as `now - start` in `resolve`'s sync branch. Commented as a deliberate addition — Go's
  `Resolved` doesn't need it since the backend never plays video — rather than silently
  diverging from the "direct port" claim without explanation.
  - Re-verified the full Node cross-check against `schedule_test.go`'s cases after this change
    (mid-item, exact start, item-longer-than-cycle, `P` not dividing `C`, empty playlist, sync
    active — now also asserting `elapsedMs` on each), plus `floorMod` — all passed, confirming
    the addition didn't disturb the existing port. Deleted the script afterward.
- `src/components/MediaFrame.jsx` — the one place that decides how each media kind renders,
  used by both the Wall tiles and the single-window route:
  - `image` → `<img>` keyed on `url` (so switching to a different image resets error state),
    `object-fit: cover`; `onError` flips to a `"label (missing)"` placeholder instead of a
    broken-image icon.
  - `video` → `<video muted autoPlay playsInline loop>`, keyed on `url`. Comment explains all
    three non-`loop` attributes are required together: browsers block autoplay unless muted,
    and without `playsInline` iOS Safari forces fullscreen instead of playing inline — there's no
    user interaction available on a wall display to hang a "click to play" prompt on, so neither
    can be dropped. `onLoadedMetadata` seeks once (guarded by a ref so it only fires on the
    initial mount for this item, not on every tick) to `elapsedMs / 1000`, modulo the video's own
    `duration` in case the clip is shorter than its scheduled slot — so a reload landing 6s into
    a 15s item resumes 6s in rather than restarting, the same clock-derived "nothing is stored"
    rule as the rest of the schedule. `onError` falls back the same way as the image case.
  - `blank` (`resolved.blank` **or** a configured `kind: "blank"` media item) → a plain dark
    panel with a label — comment distinguishes the two cases (nothing-to-show vs. a deliberate
    blank item) even though they render identically, since a future change might want to tell
    them apart visually.
- `src/windowSchedule.js` (new): adapter layer between `/api/state`'s snake_case JSON and
  `schedule.js`'s plain camelCase shapes (`toScheduleItems`, `toScheduleSync`,
  `resolveWindowMedia`) — kept out of `schedule.js` on purpose so that file stays API-shape-
  agnostic. `resolveWindowMedia(state, window, now)` is the one function both pages call to
  avoid re-deriving anchor/sync/media-lookup logic twice.
- `src/useSequencerState.js` (new): the fetch-once + clock-sync + shared-250ms-ticker logic
  pulled out of `Wall.jsx` into a hook so `SingleWindow.jsx` reuses the exact same one instead of
  rolling a second ticker — this is what "both use the same shared ticker" means in practice:
  one hook, so there's structurally no way for a second page to reintroduce per-window timers.
- `src/pages/Wall.jsx`: now uses the hook + adapter above; each tile is wrapped in a
  `react-router-dom` `<Link to={\`/window/${window.id}\`}>` so every tile is a discoverable link
  to its own single-window view.
- `src/pages/SingleWindow.jsx` (new): full-bleed `MediaFrame` for one window (`useParams()` for
  `:id`, 404-style message if no window matches) plus a small caption bar with the window name,
  a `SYNC` badge when active, and a back-link to `/` (not explicitly requested, but free to add
  and avoids a dead-end full-bleed page with no way back other than the browser's own button).
- `src/App.jsx`: added `react-router-dom` (`BrowserRouter`/`Routes`/`Route`), `/` → `Wall`,
  `/window/:id` → `SingleWindow`.
- `index.css`: `.window-tile-link` (block-level, no default link styling, a focus/hover outline
  since tiles are now interactive), `.media-video` (same `object-fit: cover` treatment as
  images), `.media-blank` (unified panel style, replacing the old two-class placeholder pattern
  now that image/video/blank all funnel through `MediaFrame`), `.single-window` /
  `.single-window-media` / `.single-window-caption` / `.single-window-back` for the full-bleed
  route.
- Verified against a **live** local backend + Vite dev server together: `npm run lint` and
  `npm run build` both clean (33 modules, up from 21). Both `/` and `/window/1` return 200 from
  the dev server; every new/changed module
  (`App.jsx`/`Wall.jsx`/`SingleWindow.jsx`/`WindowTile.jsx`/`MediaFrame.jsx`/
  `useSequencerState.js`/`windowSchedule.js`/`schedule.js`) transforms with no error; `/api/state`
  still returns all four windows correctly through CORS. Deliberately checked what happens when
  `<video src="/media/m5.mp4">` (the missing file) is requested through the dev server: it
  returns **200 with `Content-Type: text/html`** (Vite's dev SPA fallback serves `index.html` for
  any unmatched path, exactly like the production rewrite Step 11 will add) rather than a clean
  404 — confirmed this is exactly the shape of failure `MediaFrame`'s `onError` is built to
  catch, since a browser fed HTML as video data fires a decode `error` event, not a silent hang.
  Stopped both dev servers and freed their ports afterward. **Not verified** (same limitation as
  Step 8): actual in-browser rendering, the visual seek-on-reload behavior, and click-through
  navigation from a Wall tile to its `/window/:id` — no browser automation tool is available
  here, so that's left for the user's own browser testing, which is what they asked for.
