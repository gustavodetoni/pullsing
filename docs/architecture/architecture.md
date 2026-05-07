# Pullsing Architecture

## Status

Este documento descreve o estado atual do MVP no código e separa explicitamente o que já existe do que ainda é roadmap.

## O que existe hoje

- Um único binário Go em `cmd/server/main.go`.
- Admin API HTTP/JSON em `internal/interfaces/http` com endpoints para:
  - `POST /v1/projects`
  - `POST /v1/projects/{id}/environments`
  - `POST /v1/environments/{id}/flags`
  - `POST /v1/environments/{id}/api-keys:rotate`
  - `GET /healthz` e `GET /readyz`
- Camada de aplicação em `internal/application` com `AdminService`.
- Domínio mínimo para `Project`, `Environment` e `Flag` booleana em `internal/domain`.
- Persistência em PostgreSQL via `pgx` em `internal/infrastructure/postgres`.
- Publicação de eventos de mudança em Redis Pub/Sub em `internal/infrastructure/redis`.
- Contrato protobuf/gRPC em `proto/pullsing/v1/sdk.proto`.
- Servidor gRPC em `internal/interfaces/grpc` com `GetSnapshot` e `StreamUpdates`.
- Subscriber Redis + hub interno para fanout de updates aos clientes conectados.
- Ambiente local com `docker-compose.yml` para `postgres`, `redis` e `server`.

## Fluxo atual

1. O processo sobe, abre conexão com Postgres e Redis e executa migrations SQL locais.
2. A Admin API recebe comandos de escrita.
3. O `AdminService` valida a entrada com o domínio.
4. O repositório Postgres persiste projetos, ambientes, API keys e flags.
5. Na criação de flag, o servidor incrementa `environments.revision` e grava a mesma revisão na flag.
6. Depois da persistência, o servidor publica um evento JSON no canal Redis `pullsing.environment-updates`.
7. O relay Redis do servidor consome o evento e o repassa ao hub interno por ambiente.
8. Clientes gRPC autenticados recebem backlog por revisão e updates em tempo real.

## Modelo atual de dados

- `projects`: chave global única e nome.
- `environments`: pertence a projeto, tem chave única por projeto e `revision`.
- `api_keys`: armazena apenas `token_hash`; rotação revoga a chave ativa anterior.
- `flags`: hoje suporta apenas flag booleana, com `enabled`, `value_boolean` e `revision`.

## Limites do estado atual

- A autenticação do SDK usa `env_api_key` carregando a API key do ambiente no payload gRPC; ainda não há metadata/header dedicado.
- Não há autenticação/autorização na Admin API.
- Não há CRUD completo: hoje o código cobre apenas criação de projeto, ambiente, flag e rotação de API key.
- O stream incremental reconstrói o estado final desde uma revisão usando a tabela atual de flags; um changelog dedicado ainda não existe.

## Roadmap já indicado pelo repositório

- Implementar a interface gRPC `SDKService` já definida em protobuf.
- Entregar snapshot inicial por ambiente e stream de atualizações por revisão.
- Fazer cada instância do servidor consumir eventos do Redis e repassar updates aos clientes gRPC conectados.
- Implementar o SDK Go com cache local e avaliação local usando snapshot imutável.
- Expandir a API administrativa além dos comandos de criação.

## Visão arquitetural do MVP

- Fonte de verdade: PostgreSQL.
- Barramento de invalidação/fanout entre instâncias: Redis Pub/Sub.
- Interface administrativa: HTTP/JSON.
- Interface para SDKs: gRPC, conectada ao servidor com snapshot + stream incremental.
- Estratégia de consistência para clientes: revisão monotônica por ambiente.

## Testes existentes

- Testes unitários da aplicação cobrindo publicação de evento Redis ao criar flag.
- Testes HTTP cobrindo o handler de criação de flag.
- Testes gRPC cobrindo autenticação, backlog inicial e update em tempo real.

Isso indica que o projeto já valida as fatias “escrita administrativa + persistência + publicação” e “distribuição para SDK + avaliação local”, embora ainda faltem cenários de integração com infraestrutura real.
