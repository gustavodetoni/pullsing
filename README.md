# Pullsing

Pullsing é uma plataforma de **feature flags** e **remote configuration** pensada para entregar o essencial com baixa latência, topologia simples e uma experiência de desenvolvimento direta.

O foco do MVP:

- **Avaliação local no SDK** para evitar rede no hot path da aplicação
- **Atualização em tempo real** por streaming gRPC
- **PostgreSQL** como source of truth
- **Redis** para cache e fanout de eventos
- **Go** no servidor e no SDK Go

## Como funciona (fluxo)

```mermaid
flowchart LR
  A["Admin API - HTTP"] -->|cria e atualiza flags| B[(PostgreSQL)]
  A -->|publica evento| C["Redis PubSub"]

  C -->|evento| D["Servidor: subscriber e fanout"]
  D -->|empurra update| E["gRPC StreamUpdates"]

  F["SDK Go"] -->|bootstrap| G["gRPC GetSnapshot"]
  G -->|snapshot| F
  E -->|updates incrementais| F
  F -->|avaliação local O1| H["Aplicação"]
```

## Exemplo fluxo

```mermaid
flowchart LR
  subgraph Server["Servidor Pullsing - Go"]
    Admin["Admin API - HTTP/JSON"]
    SDKAPI["SDK API - gRPC"]
    Sub["Redis subscriber"]
  end

  Postgres[(PostgreSQL)]
  Redis["Redis PubSub"]
  Client["Aplicação"]
  SDK["SDK Go"]

  Client --> SDK

  Admin -->|CRUD de recursos| Postgres
  Admin -->|publica mudanças| Redis

  Sub -->|consome eventos| Redis
  Sub -->|fanout interno| SDKAPI

  SDK -->|GetSnapshot| SDKAPI
  SDKAPI -->|snapshot| SDK
  SDK -->|StreamUpdates| SDKAPI
  SDKAPI -->|updates incrementais| SDK
```

## Quickstart

O caminho mais curto para subir a stack local está em [docs/quickstart.md](docs/quickstart.md).

Resumo rápido:

```bash
make up
curl http://localhost:8080/healthz
```

Para rodar sem Docker Compose:

```bash
docker compose up -d postgres redis
PULLSING_POSTGRES_URL='postgres://pullsing:pullsing@localhost:5432/pullsing?sslmode=disable' \
PULLSING_REDIS_ADDR='localhost:6379' \
go run ./cmd/server
```

## Documentação

- [Quickstart](docs/quickstart.md)
- [Arquitetura](docs/architecture/architecture.md)
- [SDK Go](docs/sdk/sdk-go.md)
- [Benchmarks](docs/benchmarks/benchmark.md)
- [ADRs](docs/adr/README.md)

## Estrutura do repositório

```text
cmd/server/                 entrypoint do servidor
internal/domain/            entidades e invariantes
internal/application/       casos de uso do admin
internal/interfaces/http/   API HTTP/JSON do admin
internal/infrastructure/    Postgres, Redis e configuração
proto/pullsing/v1/          contrato protobuf do SDK
sdk/go/                     modulo separado do SDK Go
migrations/                 schema SQL inicial
docs/                       arquitetura, quickstart, SDK, benchmarks e ADRs
```

## API admin do MVP

Endpoints atuais:

- `GET /healthz`
- `GET /readyz`
- `POST /v1/projects`
- `POST /v1/projects/{project_id}/environments`
- `GET /v1/environments/{environment_id}/flags`
- `POST /v1/environments/{environment_id}/flags`
- `GET /v1/environments/{environment_id}/flags/{flag_id}`
- `PATCH /v1/environments/{environment_id}/flags/{flag_id}`
- `DELETE /v1/environments/{environment_id}/flags/{flag_id}`
- `POST /v1/environments/{environment_id}/api-keys:rotate`

Exemplo de criação de flag booleana:

```bash
curl -X POST http://localhost:8080/v1/environments/1/flags \
  -H 'content-type: application/json' \
  -d '{
    "key": "checkout-redesign",
    "name": "Checkout redesign",
    "description": "Libera o novo checkout",
    "enabled": true,
    "bool_value": true
  }'
```

## Desenvolvimento

Comandos principais:

```bash
make fmt
make test
make build
make proto
```

Os testes atuais passam com:

```bash
go test ./...
```
