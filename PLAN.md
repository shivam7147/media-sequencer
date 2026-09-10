# Multi-Window Media Sequencer — End-to-End Build Plan
**EVA Bharat Backend Intern, Round 2**
Deadline: **12 AM Saturday 12 September 2026** — i.e. end of Friday night. Today is Thursday. **Two days.**

---

## 0. The one idea this project rests on

Do not think about *playback*. Think about a **schedule**.

A naive build gives each window an array and a `setTimeout` that steps through it, and makes sync a broadcast that says "everyone show M2 now". It demos fine and breaks under exactly what an evaluator does: refresh one window and it restarts at item 0 while the others are mid-list; leave it running and the timers drift; trigger sync twice and the state tangles.

Instead, **what a window is showing is a pure function of the current time**:

```
currentItem(window, now) = f(cycle_anchor, playlist, cycle_length, now)
```

Nothing stores "where we are" — it is derived, every time, from the clock. That single decision buys you:

- **Refresh-proof.** A reloaded window recomputes and lands exactly where its neighbours are.
- **Drift-proof.** Windows don't stay in sync by talking to each other; they stay in sync because they all compute from the same server clock.
- **Sync for free.** A sync is an *overlay*, not a state change: the server records `{media_id, start_at, duration}`. While `now` falls in that interval every window renders that item; when it ends they fall back to the schedule that never stopped running underneath. "Resume without losing its playlist configuration" needs no bookkeeping, because nothing was ever mutated.

Say this in the README and say it in the discussion round. It *is* the assignment.

---

## 1. Locked decisions

| Area | Choice | Why |
|---|---|---|
| Backend | Go 1.25 + `chi` | Same as round 1; already proven end to end |
| Storage | **Postgres** (Render free instance), `jackc/pgx/v5` stdlib adapter | "Persistent storage usage" is a graded criterion, and playlist edits must survive restarts |
| Live updates | **SSE** (`text/event-stream`) | One-way server→client; half the code of WebSockets |
| Frontend | **React via Vite, plain JavaScript** | Fast, standard, no TS friction on a 2-day budget |
| Deployment | **Two Render services**: Static Site (React) + Web Service (Go, Docker) + Postgres | The brief lists frontend and backend deployments separately |
| Media | Bundled assets in the frontend, seeded as labelled cards **M1…M6** | No external hosts to fail; labels make sync visually obvious |
| Repo | One repo, `backend/` and `frontend/` | Simpler to submit and review than two |

**Local development uses the Render Postgres directly** via its external connection string. No local database to install.

---

## 2. The schedule maths — get this exactly right

Definitions, per window:

- `anchor` — a single global timestamp, stored once, when the cycle epoch begins
- `items` — ordered playlist `[(media₁, d₁), (media₂, d₂), … (mediaₙ, dₙ)]`, durations in seconds
- `P` — playlist duration, `Σ dᵢ`
- `C` — cycle length, default `18000` (5 hours), configurable

To find what window W shows at time `t`:

```
elapsed   = (t − anchor) mod C          // position within the 5-hour cycle
offset    = elapsed mod P               // position within the looping playlist
walk items accumulating durations until offset falls inside item k
→ item k, with (start of k + dₖ − offset) seconds remaining
```

Then clamp: if the item would run past the end of the cycle, cut it at the boundary so every window restarts its cycle together.

**Edge cases, decide now and document:**

| Case | Behaviour |
|---|---|
| Empty playlist | Show blank. Never crash. |
| `P > C` | Playlist is truncated at the cycle boundary; the tail never plays. Document it. |
| `P` doesn't divide `C` | The final repetition is cut short at the boundary. That is intended — it keeps cycles aligned. |
| Blank in playlist | An ordinary item with its own duration. **Blank never appears by default** — only when configured. |
| Playlist changed mid-cycle | Recompute immediately from the same anchor. Windows may jump; that's correct and expected. |

**Sync overlay**, applied after the above:

```
if a sync event exists with start_at ≤ now < start_at + duration:
    every window shows sync.media, with (start_at + duration − now) remaining
else:
    each window shows its own computed item
```

---

## 3. Data model

```sql
media          (id, label, kind ['image'|'video'|'blank'], url, default_duration_seconds)
windows        (id, name, position)
window_items   (id, window_id, media_id, position, duration_seconds)
sync_events    (id, media_id, start_at, duration_seconds, created_at)
settings       (key, value)        -- 'cycle_seconds', 'anchor'
```

`window_items.position` orders the playlist. `settings` holds the anchor and cycle length so both are changeable without a redeploy — which matters for the demo control below.

**Seed data:** 4 windows, 6 media items (`M1`–`M6`: four labelled colour cards, one short video, one blank), each window given a different subset in a different order, so the windows are visibly out of phase until you press sync.

---

## 4. API contract

You design this one — the brief says so. Keep it small.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | `{"status":"ok"}` |
| `GET` | `/api/time` | `{"server_time":"…RFC3339Nano"}` — clock sync |
| `GET` | `/api/state` | Everything a client needs in one call: server time, anchor, cycle seconds, all windows with their items, the media library, and the active sync if any |
| `POST` | `/api/windows/{id}/items` | `{media_id, duration_seconds?}` → append to that window's playlist |
| `DELETE` | `/api/windows/{id}/items/{itemId}` | Remove an item |
| `POST` | `/api/media` | `{label, kind, url, default_duration_seconds}` → add to library |
| `POST` | `/api/sync` | `{media_id, duration_seconds}` → start a sync now |
| `PUT` | `/api/settings/cycle` | `{cycle_seconds}` → demo control (see below) |
| `GET` | `/api/events` | SSE stream: `state_changed`, `sync` |

Errors keep round 1's shape: `{"error":"message"}`.

**The demo control is not optional.** Nobody can watch a 5-hour cycle. `PUT /api/settings/cycle` plus a control in the UI lets a reviewer set the cycle to 60 seconds and *watch* windows loop and restart together. It is the same evaluator-friendliness move the smoke test was last time — build it in, and say in the README that the default is 5 hours and this exists so the behaviour can be observed in a minute.

---

## 5. Repo layout

```
media-sequencer/
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/          # env: PORT, DATABASE_URL, ALLOWED_ORIGIN
│   │   ├── database/        # pgx pool, migrations, seed
│   │   ├── models/          # Media, Window, WindowItem, SyncEvent
│   │   ├── schedule/        # ← the core. pure functions, heavily tested
│   │   ├── store/           # all SQL
│   │   ├── sse/             # subscriber registry + broadcast
│   │   ├── middleware/      # CORS, logging, recovery
│   │   └── handlers/
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── api.js           # fetch wrappers + SSE subscription
│   │   ├── clock.js         # server-time offset estimation
│   │   ├── schedule.js      # same maths as Go, for rendering
│   │   ├── components/      # WindowTile, MediaFrame, Controls, SyncBar
│   │   └── pages/           # Wall (grid), SingleWindow
│   ├── public/media/        # bundled M1…M6 assets
│   └── package.json
└── README.md
```

`internal/schedule` is the heart. It is pure Go — no database, no HTTP — so it can be tested exhaustively. That's also what makes it easy to defend in review.

---

## 6. Execution pipeline

Same rhythm as round 1: one prompt, then **you verify before moving on**. Deadline is tighter, so the discipline matters more, not less.

Every prompt should begin: *"This is the Multi-Window Media Sequencer project. Read CLAUDE.md and PLAN.md first."*

---

### Step 1 — Repo, backend skeleton, `/health`
Scaffold `backend/` per the layout, `go mod init github.com/shivam7147/media-sequencer/backend`, chi router, config from env, graceful shutdown, `/health`, `.gitignore`, `.env.example`.
**Verify:** `go run ./cmd/server` → `curl localhost:8080/health` → `{"status":"ok"}`

### Step 2 — Postgres: provision, connect, migrate, seed
Create the free Postgres instance on Render **first** (you do this in the browser), copy the external connection string into `.env`. Then: pgx pool, `CREATE TABLE IF NOT EXISTS` migrations run at boot, and idempotent seed data (4 windows, 6 media, different playlists).
**Verify:** server boots, tables exist, seed rows present, restarting doesn't duplicate anything.
> **Do this today.** Provisioning is the one step that can surprise you, and finding out Friday night is the bad outcome.

### Step 3 — The schedule engine + exhaustive tests
`internal/schedule`: `CurrentItem(anchor, items, cycleSeconds, now) → (item, remaining)`. Pure functions. Table-driven tests covering: mid-item, exact boundaries, wrap at end of playlist, wrap at end of cycle, empty playlist, single item, `P > C`, `P` not dividing `C`, and `now` before the anchor.
**Verify:** `go test ./internal/schedule/... -v`. **Do not move on until this is green.** Everything else depends on it being right.

### Step 4 — Store + read API
SQL for media, windows, items, sync events, settings. Then `GET /api/state` and `GET /api/time`.
**Verify:** `curl /api/state` returns four windows with items and sensible timestamps.

### Step 5 — Write API
Add item, delete item, add media, trigger sync, set cycle length. Validate input; reject unknown media ids with 404.
**Verify:** add an item via curl, re-read `/api/state`, confirm it's there and survives a server restart.

### Step 6 — SSE
`GET /api/events`: register subscriber, heartbeat comment every ~20s to keep proxies from closing it, clean removal on disconnect. Broadcast `state_changed` after any write and `sync` when a sync starts.
**Verify:** `curl -N localhost:8080/api/events` in one window, POST an item in another, watch the event arrive.

### Step 7 — Dockerfile + deploy the backend
Multi-stage, `golang:1.25-alpine` → `alpine:3.20`, `CGO_ENABLED=0`, non-root. Push, create the Render Web Service, set `DATABASE_URL` and `ALLOWED_ORIGIN`, deploy.
**Verify:** `curl https://<backend>.onrender.com/api/state` returns real data.
> **Backend live by end of Thursday.** That's the target.

### Step 8 — React shell + clock + rendering
Vite app; `clock.js` estimates server offset (sample `/api/time` ~5 times, keep the lowest round-trip, offset = server − (local + rtt/2)); `schedule.js` mirrors the Go maths; the Wall page renders a grid of windows each showing its current item.
**Verify:** windows show different media and change on their own. Set cycle to 60s and watch them loop.

### Step 9 — Playback, media types, single-window route
`MediaFrame` renders image / video (muted, autoplay, loop) / blank. Each window schedules a timer for exactly its remaining seconds, then recomputes — never a fixed interval. Add `/window/:id`.
**Verify:** open `/window/1` and `/window/2` in two browser windows next to the wall. All three agree. **Refresh one — it must land back in step.**

### Step 10 — SSE wiring + controls
Subscribe to `/api/events`, recompute on every event. Controls panel: media library, add media to a window, trigger sync (pick item + duration), cycle-length control for the demo.
**Verify:** trigger sync — every window and every open browser window flips to the same item simultaneously, then each resumes its own sequence.

### Step 11 — Deploy the frontend + CORS
Render Static Site: root `frontend`, build `npm ci && npm run build`, publish `dist`, env `VITE_API_BASE`. Add a rewrite rule `/*` → `/index.html` so `/window/2` doesn't 404 on refresh. Set `ALLOWED_ORIGIN` on the backend to the static site's URL and redeploy.
**Verify:** the deployed frontend drives the deployed backend. Two browser windows on the live URL stay in sync.

### Step 12 — README, final checks, submit
README covering: what it does; **how the sync model works** (the schedule idea — give this real space); both live URLs; setup and deployment steps; environment variables; API documentation; seed data explanation; the cycle-length demo control and how to use it; assumptions and tradeoffs.
Then: `go test ./...`, `go vet ./...`, `gofmt -l .`, repo public, both URLs live, form submitted.

---

## 7. Two-day schedule

| When | Steps |
|---|---|
| **Thursday** | 1–7 — backend complete and **deployed**, schedule engine tested |
| **Friday AM** | 8–10 — React, playback, sync working locally |
| **Friday PM** | 11–12 — frontend deployed, README, verify, submit |

If Friday evening runs short, the things to cut are the delete-item endpoint and UI polish. **Never** cut: the schedule tests, the deployment, or the README's explanation of the sync model.

---

## 8. Traps specific to this build

1. **Client clock vs server clock.** Never use `Date.now()` raw. Every computation goes through the measured offset, or windows on different machines disagree.
2. **`setInterval` for playback.** Don't. Schedule one timer for exactly the remaining seconds of the current item, then recompute. Intervals drift and compound.
3. **Browser autoplay policy.** Video will not autoplay with sound. Set `muted`, `playsInline` and `autoplay`, or videos silently never start.
4. **SSE behind a proxy.** Without a periodic heartbeat, Render's proxy will close an idle stream. Send a comment line every ~20 seconds and reconnect on the client.
5. **CORS.** Two origins now. Set `ALLOWED_ORIGIN` explicitly on the backend rather than `*`, and remember SSE needs the header too.
6. **SPA routing on a static host.** Without the `/*` → `/index.html` rewrite, refreshing `/window/2` returns 404.
7. **Duplicate seeding.** Seed must be idempotent, or every restart adds four more windows.
8. **The 5-hour default.** Ship the default as 18000 seconds as specified. The demo control changes it at runtime — don't quietly ship 60 seconds and hope nobody notices.

---

## 9. Questions you will be asked in the discussion round

Have answers ready. Each one has a good answer *because of* the architecture, not despite it.

- What happens if I refresh one window?
- What if two people trigger sync at the same time?
- How do you handle a client whose clock is wrong?
- Why compute the schedule on both server and client rather than pushing "now play X"?
- What happens when someone adds media mid-cycle?
- Why Postgres here when you used SQLite last time?
- How would this behave with 50 windows instead of 4?
