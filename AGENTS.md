# Pullsing — Project Context (for agents)

Pullsing is a small, senior-level, open-source platform for **Feature Flags** and **Remote Configuration**, inspired by LaunchDarkly/Unleash but optimized for **simplicity, performance, and developer experience**.

## Goals (MVP)
- **Low-latency flag evaluation** in clients (SDK-first): evaluation must be local, O(1), no network on hot path.
- **Realtime updates** from server to SDK via **gRPC streaming**.
- **Redis (MVP)** for cache + Pub/Sub fanout (no Kafka, no k8s in MVP).
- **PostgreSQL** as the source of truth for projects/environments/flags/audit.
- **Go** as primary language for server and Go SDK; modern idiomatic Go, clean architecture without overengineering.
- **Docker Compose** for local dev (Postgres + Redis + server).
- Tests from day one (unit + basic integration).

## Non-goals (MVP)
- Microservices architecture (start as a single binary/service).
- Advanced targeting / experiments / A/B tests.
- Complex RBAC (start simple, API-key per environment).
- Kafka, Kubernetes.
- Multi-region replication (design should not block it later, but do not build now).

## Product model (MVP)
Entities (suggested):
- **Project**: top-level container.
- **Environment**: `dev/staging/prod` (per project) with its own **API key**.
- **Flag**: key + type + enabled/value; keep MVP to `bool` first, then expand.

Semantics:
- SDK reads a **snapshot** (full set) + then receives **incremental updates**.
- SDK evaluates locally using the last valid snapshot; network failures must not break evaluation.

## Repository layout (agreed)
We will use the following structure (keep it lean; avoid creating packages that do not carry real code yet):

- `cmd/server/main.go`: server entrypoint.
- `internal/domain/...`: pure domain (entities, value objects, domain errors, invariants).
- `internal/application/...`: use-cases (services) orchestrating domain + ports.
- `internal/interfaces/{http,grpc,middleware}`: transport adapters (handlers, interceptors).
- `internal/infrastructure/...`: implementations for Postgres/Redis/gRPC/HTTP wiring, repositories, caching.
- `sdk/go/...`: Go SDK as its own module (`sdk/go/go.mod`) with `client/cache/stream/evaluation/types/examples`.
- `proto/...` + `proto/gen/...`: protobuf sources + generated code.
- `migrations/`: SQL migrations for Postgres.
- `deployments/docker-compose/` + root `docker-compose.yml`: local environment.
- `tests/{integration,e2e,load}`: black-box tests and benchmarks.
- `docs/{architecture,sdk,benchmarks,adr}`: short, opinionated docs and ADRs.
- `scripts/{proto,dev}`: tooling (protoc generation, dev helpers).

We may use `go.work` to develop the root module + `sdk/go` module together locally.

## Architecture constraints
- Server is a single Go service with two interfaces:
  - **Admin API** (CRUD flags, projects, envs, keys) — HTTP/JSON for MVP simplicity.
  - **SDK API** (gRPC) — snapshot + server-streaming updates.
- **Redis Pub/Sub** propagates changes across server instances; each instance pushes updates to its connected gRPC clients.
- Server may keep a small **in-memory index** (optional in MVP) for hot reads; always persist to Postgres and publish invalidation/events to Redis.

## Performance principles
- No per-request DB hits on SDK hot path (SDK evaluates locally).
- Server must avoid per-client expensive work:
  - Send compact snapshots, support compression.
  - Use monotonic versioning (revision) per environment to avoid full diff computation.
- Prefer immutable data structures in SDK cache (atomic swap of snapshot).

## Coding standards
- Idiomatic Go, `context.Context` everywhere, explicit errors.
- Keep packages small and purpose-driven.
- Avoid generic “frameworks”; use standard library + minimal dependencies.
- Prefer table-driven tests; integration tests via Docker Compose where feasible.

## Deliverables (incremental)
We will implement in small slices:
1) protobuf + gRPC service contract
2) minimal server skeleton + storage schema
3) CRUD + publishing updates
4) SDK snapshot + stream + local evaluation
5) integration tests + docs polish
   
- Prioritize performance, simplicity, and idiomatic code in Go.
- Don't create microservices in the MVP.
- Don't use Kafka/Kubernetes in the MVP.
- Use PostgreSQL for persistence.
- Use Redis for caching and Pub/Sub.
- Use gRPC Streams for real-time.
- The Go SDK should have local caching and local evaluation.
- Every significant change should be tested.