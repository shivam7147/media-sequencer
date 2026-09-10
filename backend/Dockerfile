# Build context is backend/ (not the repo root) — every path below is
# relative to that directory.

# ---- Stage 1: build --------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src

# Separate layer so `go mod download` is only re-run when dependencies
# actually change, not on every source edit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0: pgx (github.com/jackc/pgx/v5) is a pure-Go Postgres
# driver — it speaks the wire protocol itself rather than linking libpq —
# so this needs no C toolchain and produces a fully static binary that
# runs unmodified on the minimal alpine base in stage 2.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# ---- Stage 2: run ------------------------------------------------------
FROM alpine:3.20

# The database connection uses TLS (sslmode=require) — without this, pgx
# has no root certificates to verify the server against and every
# connection attempt fails.
RUN apk add --no-cache ca-certificates

RUN addgroup -S app && adduser -S -G app app

COPY --from=build /app/server /app/server

USER app

EXPOSE 8080

# All application state lives in Postgres, so there's no data directory
# to declare here — the container itself is disposable.

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O- http://localhost:8080/health || exit 1

CMD ["/app/server"]
