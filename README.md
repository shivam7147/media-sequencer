# Multi-Window Media Sequencer

Multiple display windows each play their own media playlist continuously on a
5-hour cycle. A sync action shows one chosen media item across every window at
the same instant, after which each window resumes its own sequence.

**Live app:** https://media-sequencer-xtb5.onrender.com
**API:** https://media-sequencer-api.onrender.com — health: `/health`, full state: `/api/state`

Author: Shivam Rao — [GitHub](https://github.com/shivam7147) · [LinkedIn](https://www.linkedin.com/in/shivam-rao-940327290/)

## How the sync model works

What a window shows is **a pure function of the current time**, not stored state:

    currentItem = f(cycle_anchor, playlist, cycle_length, now)

Nothing records "where we are" — it is derived on every tick from the server's
clock. Consequences:

- **Refresh-proof.** A reloaded window recomputes and lands exactly where the
  others are, rather than restarting at item 1.
- **Drift-proof.** Windows don't stay in sync by talking to each other; they
  agree because they all compute from the same clock. Clients measure their
  offset from the server via `/api/time` (five samples, lowest round-trip wins).
- **Sync is an overlay, not a mutation.** `POST /api/sync` records
  `{media_id, start_at, duration}`. While `now` is inside that interval every
  window renders that item; when it ends they fall back to the schedule that
  never stopped running underneath — so "resume without losing playlist
  configuration" needs no bookkeeping.
- The 5-hour cycle is the modulus: the playlist repeats to fill it, and blank
  appears only when blank is an actual playlist item.

The maths lives in `backend/internal/schedule` as pure, unit-tested Go, and is
ported line-for-line to `frontend/src/schedule.js`. `/api/state` includes a
server-computed `resolved` block per window, so the schedule can be verified
with `curl` alone.

## Stack

Go 1.25 + chi · PostgreSQL (pgx) · Server-Sent Events · React (Vite) ·
Docker · deployed on Render (API as a web service, frontend as a static site).

## Run locally

Backend — needs `DATABASE_URL` in `backend/.env` (see `.env.example`):

    cd backend && go run ./cmd/server      # :8080

Frontend — `VITE_API_BASE` in `frontend/.env`:

    cd frontend && npm install && npm run dev   # :5173

Docker: `cd backend && docker build -t media-sequencer-api . && docker run -p 8080:8080 --env-file .env media-sequencer-api`

Tests: `cd backend && go test ./...`

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | `{"status":"ok"}` |
| GET | `/api/time` | server clock, for offset estimation |
| GET | `/api/state` | windows, playlists, media library, active sync, resolved item per window |
| POST | `/api/windows/{id}/items` | add media to a window |
| DELETE | `/api/windows/{id}/items/{itemId}` | remove an item |
| POST | `/api/media` | add to the media library |
| POST | `/api/sync` | start a sync now |
| PUT | `/api/settings/cycle` | set cycle length (demo aid) |
| GET | `/api/events` | SSE stream of `state_changed` / `sync` |

Errors are `{"error":"..."}`.

## Seed data

The brief referred to example windows and media lists but did not include them,
so I defined my own: 4 windows and 6 media items (M1–M4 labelled image cards,
M5 a short video, M6 blank), each window holding a different subset in a
different order so the windows are visibly out of phase until a sync fires.
Seeding is idempotent and gated inside a transaction.

## Demo aid: cycle length

The cycle defaults to 18000 seconds (5 hours) as specified. Nobody can watch
that, so `PUT /api/settings/cycle` (clamped to 10–86400) changes it at runtime:

    curl -X PUT https://media-sequencer-api.onrender.com/api/settings/cycle \
      -H "Content-Type: application/json" -d '{"cycle_seconds":60}'

Set it to 60 and all four windows visibly loop and restart together every
minute. Remember to set it back to 18000.

## Assumptions and current state

- Ownership of correctness sits server-side; the client recomputes the same
  maths purely for smooth rendering.
- A playlist longer than the cycle is truncated at the cycle boundary; a
  playlist that doesn't divide the cycle evenly has its final repetition cut
  short. Both keep every window's cycle aligned.
- Concurrent syncs: last write wins, deterministically, because the active sync
  is a query ordered by `start_at`, not a state machine.
- Video autoplay requires `muted` + `playsInline`; a reload mid-clip seeks to
  the correct offset rather than restarting.
- The wall page has a controls panel (add an item to a window, trigger a
  sync overlay, change cycle length) and subscribes to `/api/events` so
  every open browser refetches `/api/state` when something changes. The
  same curl calls still work if you want to drive it from the API.
- Hosting is Render's free tier: the API sleeps after ~15 minutes idle, so the
  first request may take ~50 seconds, and the free Postgres instance expires
  after 30 days.
- The static site rewrites `/*` to `/index.html` (see `render.yaml`) so a
  refresh of `/window/:id` does not 404.
