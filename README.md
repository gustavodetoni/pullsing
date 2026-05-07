# Pullsing

Pullsing e uma plataforma de **feature flags** e **remote configuration** pensada para entregar o essencial com baixa latencia, topologia simples e uma experiencia de desenvolvimento direta.

O foco do MVP e bem definido:

- **avaliacao local no SDK** para evitar rede no hot path da aplicacao
- **atualizacao em tempo real** por streaming gRPC
- **PostgreSQL** como source of truth
- **Redis** para cache e fanout de eventos
- **Go** no servidor e no SDK Go

Hoje o repositorio ja possui a base do servidor admin, o schema inicial no Postgres, o contrato protobuf do SDK, a publicacao de eventos via Redis, o servidor gRPC para snapshot/update e o SDK Go com bootstrap por snapshot e stream incremental.

## Estado atual do MVP

Ja existe:

- API HTTP admin para criar `projects`, `environments` e `flags`
- rotacao de API key por ambiente
- migracao inicial do Postgres
- revisao monotonicamente crescente por ambiente
- publicacao de mudancas de flag em Redis Pub/Sub
- subscriber Redis + fanout interno para clientes gRPC
- contrato protobuf em `proto/pullsing/v1/sdk.proto`
- servidor gRPC com `GetSnapshot` e `StreamUpdates`
- autenticacao do SDK via `env_key` usando a API key do ambiente
- stream com backlog por revisao e desconexao de cliente lento
- SDK Go com bootstrap, reconnect e avaliacao local
- testes unitarios da camada de aplicacao e handlers HTTP

Ainda falta:

- benchmarks medidos em ambiente reproduzivel

## Quickstart

O caminho mais curto para subir a stack local esta em [docs/quickstart.md](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/quickstart.md).

Resumo rapido:

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

## Documentacao

- [Quickstart](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/quickstart.md)
- [Arquitetura](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/architecture/architecture.md)
- [SDK Go](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/sdk/sdk-go.md)
- [Benchmarks](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/benchmarks/benchmark.md)
- [ADRs](/Users/gustavodetoni/conductor/workspaces/pullsing/pattaya/docs/adr/README.md)

## Estrutura do repositorio

```text
cmd/server/                 entrypoint do servidor
internal/domain/            entidades e invariantes
internal/application/       casos de uso do admin
internal/interfaces/http/   API HTTP/JSON do admin
internal/infrastructure/    Postgres, Redis e configuracao
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
- `POST /v1/environments/{environment_id}/flags`
- `POST /v1/environments/{environment_id}/api-keys:rotate`

Exemplo de criacao de flag booleana:

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
