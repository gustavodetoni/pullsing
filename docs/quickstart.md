# Quickstart

Este quickstart cobre o estado atual do repositorio: **API admin HTTP + gRPC SDK + Postgres + Redis**.

## Pre-requisitos

- Go `1.26`
- Docker e Docker Compose
- `curl`

## Opcao 1: stack completa com Docker Compose

Suba os servicos:

```bash
make up
```

Verifique saude:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Veja logs do servidor:

```bash
make logs
```

Derrube tudo:

```bash
make down
```

## Opcao 2: Postgres e Redis no Docker, servidor via `go run`

Suba apenas dependencias:

```bash
docker compose up -d postgres redis
```

Rode o servidor apontando para localhost:

```bash
PULLSING_POSTGRES_URL='postgres://pullsing:pullsing@localhost:5432/pullsing?sslmode=disable' \
PULLSING_REDIS_ADDR='localhost:6379' \
go run ./cmd/server
```

Isso aplica as migracoes em `migrations/` no startup e expoe:

- API admin HTTP em `:8080`
- API gRPC do SDK em `:50051`

## Fluxo basico da API admin

### 1. Criar projeto

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H 'content-type: application/json' \
  -d '{
    "key": "acme-store",
    "name": "Acme Store"
  }'
```

Resposta esperada:

```json
{
  "id": 1,
  "key": "acme-store",
  "name": "Acme Store",
  "created_at": "2026-05-06T00:00:00Z",
  "updated_at": "2026-05-06T00:00:00Z"
}
```

### 2. Criar ambiente

```bash
curl -X POST http://localhost:8080/v1/projects/1/environments \
  -H 'content-type: application/json' \
  -d '{
    "key": "dev",
    "name": "Development"
  }'
```

Resposta esperada:

```json
{
  "environment": {
    "id": 1,
    "project_id": 1,
    "key": "dev",
    "name": "Development",
    "revision": 0,
    "created_at": "2026-05-06T00:00:00Z",
    "updated_at": "2026-05-06T00:00:00Z"
  },
  "api_key": "psk_..."
}
```

Guarde a `api_key`. Ela e a credencial real que o SDK envia no campo `env_api_key`.

### 3. Criar flag booleana

```bash
curl -X POST http://localhost:8080/v1/environments/1/flags \
  -H 'content-type: application/json' \
  -d '{
    "key": "checkout-redesign",
    "name": "Checkout redesign",
    "description": "Libera o fluxo novo",
    "enabled": true,
    "bool_value": true
  }'
```

Cada criacao de flag incrementa `revision` do ambiente e publica um evento em Redis com este formato logico:

```json
{
  "environment_id": 1,
  "revision": 1,
  "changed_keys": ["checkout-redesign"]
}
```

### 4. Executar o exemplo do SDK Go

Use exatamente a `api_key` retornada no passo 2 e uma flag que exista com a mesma `key` consultada pelo exemplo. Por padrao, o exemplo le `checkout-redesign`, entao o passo 3 ja cria a flag correta. O cliente faz bootstrap via `GetSnapshot`, inicia `StreamUpdates` em background e avalia a flag localmente.

```bash
PULLSING_API_KEY='psk_...' \
PULLSING_FLAG_KEY='checkout-redesign' \
go run ./sdk/go/examples/simple
```

Se o servidor estiver ouvindo gRPC em outro endereco, ajuste `PULLSING_ADDR`. O default do exemplo e `localhost:50051`, que corresponde ao bind padrao de `PULLSING_GRPC_ADDR=:50051` no servidor.

Saida esperada:

```text
2026/05/06 00:00:00 checkout-redesign enabled=true
```

### 5. Rotacionar API key

```bash
curl -X POST http://localhost:8080/v1/environments/1/api-keys:rotate
```

## Variaveis de ambiente

- `PULLSING_APP_NAME`: nome do servico. Default `pullsing-server`
- `PULLSING_ENV`: ambiente. Default `development`
- `PULLSING_HTTP_ADDR`: bind HTTP. Default `:8080`
- `PULLSING_GRPC_ADDR`: bind gRPC. Default `:50051`
- `PULLSING_GRPC_CLIENT_BUFFER`: buffer por cliente no fanout gRPC. Default `16`
- `PULLSING_POSTGRES_URL`: URL do Postgres
- `PULLSING_REDIS_ADDR`: endereco do Redis
- `PULLSING_SHUTDOWN_TIMEOUT`: timeout de shutdown
- `PULLSING_HTTP_READ_TIMEOUT`: timeout de leitura
- `PULLSING_HTTP_READ_HEADER_TIMEOUT`: timeout de headers
- `PULLSING_HTTP_WRITE_TIMEOUT`: timeout de escrita
- `PULLSING_HTTP_IDLE_TIMEOUT`: idle timeout

## Comandos uteis

```bash
make fmt
make test
make build
make proto
```

## Limites atuais

- flags apenas do tipo `bool`
- sem targeting avancado
- sem tipos de configuracao alem de `bool`
