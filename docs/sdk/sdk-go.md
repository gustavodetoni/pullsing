# SDK Go

## Objetivo

O SDK Go do Pullsing existe para permitir avaliação local de feature flags e remote configuration com latência previsível e sem dependência de rede no hot path. O contrato esperado para o MVP é:

- carregar um snapshot inicial do ambiente via gRPC;
- manter esse snapshot atualizado por streaming de mudanças incrementais;
- avaliar flags localmente usando o último estado válido em memória;
- continuar funcionando mesmo durante falhas transitórias de rede.

Este documento descreve o contrato que já existe no repositório, as metas de implementação e o que ainda falta.

## Estado atual no repositório

Hoje o módulo `sdk/go` já existe como módulo Go separado:

```go
module github.com/gustavodetoni/pullsing/sdk/go
```

Também já existe a estrutura inicial de diretórios:

- `sdk/go/client`
- `sdk/go/cache`
- `sdk/go/stream`
- `sdk/go/evaluation`
- `sdk/go/types`
- `sdk/go/examples`

No entanto, ainda não há implementação do SDK nesses diretórios. Neste momento, o artefato mais concreto para o SDK é o contrato protobuf/gRPC definido em `proto/pullsing/v1/sdk.proto`.

## Contrato gRPC atual

O serviço exposto para o SDK é `SDKService` com dois métodos:

- `GetSnapshot(GetSnapshotRequest) returns (Snapshot)`
- `StreamUpdates(StreamUpdatesRequest) returns (stream Update)`

### Identificação do ambiente

As duas chamadas usam `env_key`:

- `GetSnapshotRequest.env_key`
- `StreamUpdatesRequest.env_key`

No estado atual do contrato, o SDK seleciona o ambiente por essa chave. Ainda existe uma lacuna a fechar entre o protobuf atual e a modelagem administrativa do servidor: hoje a Admin API gera e rotaciona `api_key` por ambiente, enquanto o contrato gRPC expõe `env_key` no request. A forma final de autenticação e identificação ainda precisa ser consolidada junto da implementação do servidor e do SDK.

### Modelo de dados de flags

O protobuf atual define:

- `Flag.key`
- `Flag.type`
- `Flag.enabled`
- `Flag.bool_value`

Para o MVP, o único tipo explicitamente definido é `FLAG_TYPE_BOOL`. Isso é coerente com a meta atual do projeto: começar com flags booleanas antes de expandir para outros tipos de configuração.

Implicação prática para o SDK:

- a avaliação local inicial pode ser focada em booleanos;
- o cache local precisa preservar ao menos `key`, `enabled`, `type` e `bool_value`;
- a API pública do SDK deve deixar clara a diferença entre "flag desabilitada" e "flag inexistente", porque o protobuf atual não define isso sozinho.

### Snapshot inicial

`Snapshot` contém:

- `revision`
- `flags`

Interpretação esperada para o SDK:

- o SDK faz `GetSnapshot` no bootstrap;
- armazena o conjunto completo de flags do ambiente;
- registra a `revision` mais recente recebida;
- usa esse estado como base para avaliação local.

O contrato não descreve ainda paginação, compressão, checksum, expiração de snapshot ou estratégia de cold start offline. Se essas capacidades forem necessárias, devem aparecer depois no contrato ou na camada de transporte.

### Atualizações incrementais

`Update` contém:

- `revision`
- `mutations`

Cada `FlagMutation` contém:

- `type`
- `key`
- `flag`

Os tipos de mutação atuais são:

- `MUTATION_TYPE_UPSERT`
- `MUTATION_TYPE_DELETE`

Interpretação esperada para o SDK:

- iniciar `StreamUpdates` com `since_revision` igual à revisão mais recente do snapshot local;
- aplicar `UPSERT` como inserção/atualização idempotente no cache local;
- aplicar `DELETE` removendo a flag pelo `key`;
- atualizar a revisão local apenas após aplicar com sucesso o `Update`.

O protobuf não define explicitamente semântica para lacuna de revisão, replay, deduplicação ou re-sync obrigatório. A implementação do SDK deve assumir que esses casos podem acontecer e ter uma estratégia clara, por exemplo:

- se o stream falhar, reconectar com a última revisão aplicada;
- se o servidor indicar inconsistência de revisão no futuro, forçar novo `GetSnapshot`;
- tratar updates fora de ordem como erro operacional e não como sucesso silencioso.

## Metas de implementação do SDK

O SDK Go ainda precisa ser implementado, mas o desenho esperado para o MVP pode seguir estes componentes.

### `types`

Tipos públicos mínimos:

- representação de flag booleana;
- representação de snapshot imutável;
- erros bem definidos para bootstrap, stream e avaliação;
- configuração do cliente.

### `cache`

Responsabilidades esperadas:

- manter o snapshot ativo em memória;
- suportar troca atômica de snapshot;
- permitir leitura lock-free ou com contenção mínima no caminho de avaliação;
- expor a revisão atual.

Uma abordagem coerente com as metas do projeto é usar snapshot imutável com atomic swap, em vez de mutação granular visível aos leitores.

### `stream`

Responsabilidades esperadas:

- abrir `StreamUpdates`;
- reconectar automaticamente;
- reaplicar `since_revision`;
- repassar updates válidos para a camada de cache;
- registrar falhas sem interromper a avaliação local.

### `evaluation`

Responsabilidades esperadas:

- avaliar `bool` localmente em O(1);
- expor fallback explícito quando a flag não existir;
- evitar qualquer acesso de rede, banco ou bloqueio pesado durante avaliação.

### `client`

Responsabilidades esperadas:

- inicialização do SDK;
- bootstrap por snapshot;
- lifecycle do stream;
- API pública para consulta de flags;
- shutdown limpo via `context.Context`.

## Comportamento esperado no MVP

Mesmo sem código ainda, estas propriedades devem orientar a implementação:

- bootstrap síncrono ou controlado: o chamador precisa saber quando há snapshot utilizável;
- avaliação local continua disponível durante indisponibilidade de rede;
- reconnect não pode apagar o último snapshot válido;
- a ausência de implementação de targeting avançado significa que a avaliação inicial deve ser direta por chave;
- a API pública deve ser pequena e previsível.

Um formato plausível de uso, ainda não implementado, seria:

```go
client, err := sdk.NewClient(ctx, sdk.Config{
    EnvKey: "env_123",
    Endpoint: "dns:///pullsing:50051",
})

enabled := client.BoolValue("checkout_new_flow", false)
```

Esse exemplo é apenas ilustrativo. O repositório ainda não define a API final do pacote.

## O que falta

Itens ausentes hoje:

- implementação do cliente gRPC do SDK;
- política de retry e backoff;
- cache local e modelo de concorrência;
- API pública de avaliação;
- suporte a shutdown e observabilidade;
- exemplos executáveis;
- testes unitários;
- testes de integração contra o servidor;
- benchmarks reais do SDK.

Também faltam decisões de produto e protocolo que impactam o SDK:

- semântica exata para flag inexistente;
- comportamento quando `enabled=false` e `bool_value=true`;
- estratégia para revisão inválida ou atrasada;
- autenticação/transporte além do `env_key`;
- extensão futura para tipos não booleanos.

## Critérios de aceitação sugeridos

Quando o SDK Go começar a ser implementado, esta documentação deve ser atualizada junto com evidências concretas. Para o MVP, um conjunto razoável de critérios é:

- conseguir bootstrap de um ambiente via `GetSnapshot`;
- manter sincronização por `StreamUpdates`;
- avaliar flags booleanas localmente sem rede;
- reconectar sem perder o último snapshot válido;
- ter testes cobrindo bootstrap, reconnect e aplicação de mutações;
- publicar benchmarks reprodutíveis em `docs/benchmarks/benchmark.md`.
