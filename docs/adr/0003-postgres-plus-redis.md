# ADR 0003: PostgreSQL as Source of Truth, Redis for Pub/Sub

- Status: Accepted
- Date: 2026-05-06

## Context

O MVP precisa persistência relacional simples e confiável para projetos, ambientes, flags e API keys, mas também precisa propagar mudanças rapidamente entre instâncias do servidor e, no futuro, para conexões gRPC ativas.

Kafka e arquitetura mais distribuída foram explicitamente deixados fora do escopo do MVP.

## Decision

Usar:

- PostgreSQL como fonte de verdade;
- Redis como mecanismo leve de publicação/assinatura para notificações de mudança.

O padrão esperado é:

1. gravar estado durável no Postgres;
2. avançar a `revision` do ambiente;
3. publicar um evento curto no Redis;
4. cada instância interessada reagir ao evento sem usar Redis como storage primário.

## Consequences

- Modelo operacional simples para desenvolvimento local e MVP.
- Estado crítico permanece transacional no banco relacional.
- Redis fica restrito a coordenação e fanout, não a consistência durável.
- Em caso de perda de mensagem Pub/Sub, a recuperação depende de rehidratação por snapshot/revisão, não do Redis.

## Current implementation status

Já existe no código:

- schema SQL para `projects`, `environments`, `api_keys` e `flags`;
- migrations executadas no bootstrap do servidor;
- store Postgres com criação de projeto, ambiente, rotação de API key e criação de flag;
- incremento transacional de `environments.revision` ao criar flag;
- publisher Redis no canal `pullsing.environment-updates`.

Ainda não existe no código:

- consumer Redis no servidor;
- repasse de eventos Redis para clientes gRPC;
- leitura incremental por revisão para reconstrução de estado no lado servidor ou SDK.

Assim, a escolha Postgres + Redis já está materializada na escrita administrativa, mas o circuito completo de distribuição ainda não foi fechado.
