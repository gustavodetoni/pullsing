# Pullsing — Technical Roadmap (phased)

This roadmap is intentionally **small and iterative**. Each phase should produce a working system, even if minimal.

## Phase 0 — Foundation (repo + contracts)
**Outcome:** project compiles, local dev environment works, contracts are stable.
- Create repo structure (aligned with `internal/domain`, `internal/application`, `internal/interfaces`, `internal/infrastructure`, plus `sdk/go`).
- Go modules:
  - Root `go.mod` for the server.
  - `sdk/go/go.mod` for the SDK (versioned independently).
  - Root `go.work` to develop both modules together.
- Define **Protobuf** for SDK API:
  - `GetSnapshot(env_api_key)` (unary) or initial message on stream
  - `StreamUpdates(env_api_key, since_revision)` (server-streaming)
  - Messages: `Flag`, `Snapshot{revision, flags}`, `Update{revision, mutations}`
- Define initial data model for Postgres (migrations):
  - `projects`, `environments`, `api_keys`, `flags`, `flag_versions` (optional), `audit_log` (optional)
- Docker Compose: Postgres + Redis + server.
- CI basics (go test, lint optional later).

## Phase 1 — Server MVP (CRUD + event propagation)
**Outcome:** admin can create environments/flags and updates propagate via Redis.
- Admin API (HTTP/JSON) for:
  - Create project/environment, rotate API key
  - CRUD flags (initially **bool flags** only)
- Storage layer in Postgres + migrations.
- Redis Pub/Sub:
  - Publish event on flag change: `{env_id, revision, changed_keys}`
  - Server instances subscribe and update their in-memory view (if enabled) and notify gRPC streams.
- Revisioning:
  - Maintain monotonically increasing `revision` per environment (transactional in Postgres).

## Phase 2 — SDK API (Snapshot + gRPC streaming)
**Outcome:** SDK clients can fetch snapshot and receive realtime updates.
- gRPC server endpoints:
  - On connect: authenticate by environment API key, send `Snapshot(revision, flags)`
  - Keep stream open; on updates, send `Update(revision, mutations)`
- Backpressure & robustness:
  - Per-connection send loop; drop/close slow consumers with clear error.
  - Heartbeats (optional) and reconnect semantics.

## Phase 3 — Go SDK (local eval + cache + resilience)
**Outcome:** application can call `client.Enabled("new_button")` with low latency.
- SDK responsibilities:
  - Local cache: immutable snapshot swapped atomically.
  - `Enabled(key)` -> O(1) map lookup.
  - Background goroutine: connect -> receive snapshot -> apply updates.
  - Reconnect with exponential backoff + jitter.
  - Fallback: keep last valid snapshot; expose health/last revision.
- Provide simple API:
  - `NewClient(ctx, Config{EnvAPIKey, Addr, DialOptions...})`
  - `Enabled(key string) bool` (and later `GetBool(key, default)`).

Notes:
- SDK code lives under `sdk/go/` (`client/`, `cache/`, `stream/`, `evaluation/`, `types/`, `examples/`).

## Phase 4 — Tests (unit + integration)
**Outcome:** confidence and regression protection.
- Unit tests for:
  - Revisioning logic, update messages, SDK cache swap, reconnection logic.
- Integration tests (docker compose):
  - Create env/flag -> SDK receives update -> `Enabled` changes.
  - Redis Pub/Sub propagation between two server instances (optional in MVP but valuable).

## Phase 5 — Docs + DX polish
**Outcome:** easy onboarding, clear constraints, stable APIs.
- README with quickstart (docker compose + example app).
- Architecture doc (short): data flow, failure modes, performance.
- Versioning policy for protobuf/SDK.

## MVP scope: included vs excluded
**Included**
- Projects, environments (dev/staging/prod), API key per environment.
- Bool flags, CRUD, snapshot + streaming updates.
- Redis Pub/Sub for update fanout.
- Go SDK with local evaluation + reconnect + fallback snapshot.

**Excluded (post-MVP)**
- Targeting rules/segments, percentage rollout, experiments.
- Multi-tenant RBAC, SSO.
- Multi-region active-active, edge caching, CDN.
- Advanced audit + analytics pipelines.
