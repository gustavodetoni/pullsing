# ADR 0001: Single Binary for the MVP

- Status: Accepted
- Date: 2026-05-06

## Context

O MVP precisa validar rápido o fluxo principal de feature flags sem introduzir custo operacional de múltiplos serviços. O repositório já está estruturado como uma aplicação Go única, com camadas internas para domínio, aplicação, interfaces e infraestrutura.

Ao mesmo tempo, o produto precisa expor duas interfaces distintas:

- Admin API via HTTP/JSON.
- SDK API via gRPC streaming.

## Decision

O MVP será entregue como um único processo Go.

Esse processo concentra:

- bootstrap de configuração;
- conexão com Postgres;
- conexão com Redis;
- execução de migrations;
- Admin API HTTP;
- futura SDK API gRPC.

## Consequences

- Menor complexidade de deploy e desenvolvimento local.
- Menos coordenação entre serviços neste estágio.
- Código precisa manter separação interna clara para não acoplar transporte, domínio e infraestrutura.
- Escala independente por interface fica adiada para depois do MVP.

## Current implementation status

Já existe no código:

- binário único em `cmd/server/main.go`;
- Admin API HTTP ativa;
- integração com Postgres e Redis no mesmo processo.

Ainda não existe no código:

- servidor gRPC ativo no mesmo binário;
- multiplexação HTTP + gRPC no processo.

Ou seja, a decisão do binário único já foi adotada, mas só a metade HTTP do desenho está implementada hoje.
