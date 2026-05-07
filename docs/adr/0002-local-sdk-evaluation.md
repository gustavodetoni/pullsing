# ADR 0002: Local Evaluation in SDKs

- Status: Accepted
- Date: 2026-05-06

## Context

O objetivo central do produto é evitar dependência de rede no hot path de avaliação. O contrato protobuf já aponta para esse modelo ao separar:

- `GetSnapshot`, para carga inicial;
- `StreamUpdates`, para atualizações incrementais por revisão.

Esse desenho permite que o servidor distribua estado e que o SDK avalie localmente com a última versão válida.

## Decision

Os SDKs do Pullsing devem avaliar flags localmente a partir de um snapshot em memória, atualizado por stream.

Princípios da decisão:

- nenhuma chamada de rede durante a avaliação normal;
- snapshot inicial completo por ambiente;
- atualizações incrementais associadas a `revision`;
- comportamento resiliente a falhas de rede, preservando a última configuração válida.

## Consequences

- A latência de avaliação fica previsível e local ao processo cliente.
- O servidor precisa entregar snapshots compactos e updates ordenados por revisão.
- O SDK precisa de cache local seguro para troca atômica de estado.
- Recursos como targeting avançado continuam fora do escopo do MVP.

## Current implementation status

Já existe no código:

- contrato protobuf para snapshot e stream em `proto/pullsing/v1/sdk.proto`;
- campo `revision` no modelo de ambiente e flag;
- incremento de revisão ao criar flag.

Ainda não existe no código:

- implementação do servidor gRPC;
- leitura de snapshot por ambiente;
- stream de updates do servidor;
- SDK Go funcional com cache e avaliação local.

Portanto, esta ADR descreve a direção principal do produto, mas a implementação ainda está majoritariamente em roadmap.
