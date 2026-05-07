# SDK Go

## Objetivo

O SDK Go do Pullsing existe para permitir avaliacao local de feature flags e remote configuration com latencia previsivel e sem dependencia de rede no hot path. No MVP atual ele ja cobre:

- carregar um snapshot inicial do ambiente via gRPC;
- manter esse snapshot atualizado por streaming de mudancas incrementais;
- avaliar flags localmente usando o ultimo estado valido em memoria;
- continuar funcionando mesmo durante falhas transitorias de rede.

Este documento descreve o que ja esta implementado no repositorio, a API publica atual e os limites conhecidos do MVP.

## Estado atual no repositorio

O modulo `sdk/go` existe como modulo Go separado:

```go
module github.com/gustavodetoni/pullsing/sdk/go
```

Ele esta organizado nos pacotes:

- `sdk/go/client`
- `sdk/go/cache`
- `sdk/go/stream`
- `sdk/go/evaluation`
- `sdk/go/types`
- `sdk/go/examples`

Os pacotes acima ja implementam:

- bootstrap por `GetSnapshot`;
- stream incremental por `StreamUpdates`;
- reconnect com reuso da ultima revisao aplicada;
- cache local em memoria;
- avaliacao local de flags booleanas;
- exemplo executavel em `sdk/go/examples/simple`.

## Contrato gRPC atual

O servico exposto para o SDK e `SDKService` com dois metodos:

- `GetSnapshot(GetSnapshotRequest) returns (Snapshot)`
- `StreamUpdates(StreamUpdatesRequest) returns (stream Update)`

### Identificacao do ambiente

As duas chamadas usam `env_api_key`:

- `GetSnapshotRequest.env_api_key`
- `StreamUpdatesRequest.env_api_key`

No estado atual do contrato, o SDK envia a API key do ambiente no campo `env_api_key`. O servidor autentica essa chave comparando o hash com a tabela `api_keys` ativa do ambiente.

### Modelo de dados de flags

O protobuf atual define:

- `Flag.key`
- `Flag.type`
- `Flag.enabled`
- `Flag.bool_value`

Para o MVP, o unico tipo explicitamente definido e `FLAG_TYPE_BOOL`. Isso e coerente com a meta atual do projeto: comecar com flags booleanas antes de expandir para outros tipos de configuracao.

Implicacao pratica para o SDK:

- a avaliacao local atual e focada em booleanos;
- o cache local preserva `key`, `enabled`, `type` e `bool_value`;
- a API publica atual para leitura e `Enabled(key string) bool`.

### Snapshot inicial

`Snapshot` contem:

- `revision`
- `flags`

Comportamento implementado no SDK atual:

- o cliente faz `GetSnapshot` durante a inicializacao;
- armazena o conjunto completo de flags do ambiente;
- registra a `revision` mais recente recebida;
- usa esse estado como base para avaliacao local.

O contrato nao descreve ainda paginacao, compressao, checksum, expiracao de snapshot ou estrategia de cold start offline. Se essas capacidades forem necessarias, devem aparecer depois no contrato ou na camada de transporte.

### Atualizacoes incrementais

`Update` contem:

- `revision`
- `mutations`

Cada `FlagMutation` contem:

- `type`
- `key`
- `flag`

Os tipos de mutacao atuais sao:

- `MUTATION_TYPE_UPSERT`
- `MUTATION_TYPE_DELETE`

Comportamento implementado no SDK atual:

- iniciar `StreamUpdates` com `since_revision` igual a revisao mais recente do snapshot local;
- aplicar `UPSERT` como insercao/atualizacao idempotente no cache local;
- aplicar `DELETE` removendo a flag pelo `key`;
- atualizar a revisao local apenas apos aplicar com sucesso o `Update`;
- reconectar em caso de falha usando a ultima revisao local conhecida.

O protobuf nao define explicitamente semantica para lacuna de revisao, replay, deduplicacao ou re-sync obrigatorio. A implementacao atual assume fluxo monotonicamente crescente; qualquer extensao desse comportamento deve ser refletida no contrato.

## API publica atual

O ponto de entrada atual e `sdk/go/client`.

### Configuracao

```go
type Config struct {
    EnvAPIKey   string
    Addr        string
    DialOptions []grpc.DialOption
    Backoff     stream.BackoffConfig
    Logger      stream.Logger
}
```

Campos obrigatorios:

- `EnvAPIKey`: API key do ambiente criada pela API admin;
- `Addr`: endereco do servidor gRPC, por exemplo `localhost:50051`.

### Cliente

```go
sdk, err := client.NewClient(ctx, client.Config{
    EnvAPIKey: envAPIKey,
    Addr:      "localhost:50051",
})
if err != nil {
    return err
}
defer sdk.Close()
```

O `NewClient` valida a configuracao, abre a conexao gRPC e inicia um loop em background responsavel por snapshot inicial, stream e reconnect.

Metodos publicos disponiveis hoje:

- `Enabled(key string) bool`: retorna o estado local da flag booleana;
- `Revision() uint64`: retorna a revisao local atual;
- `Health() Health`: expoe estado de conectividade e ultimo erro observado;
- `Close() error`: encerra o loop do stream e fecha a conexao gRPC.

### Health

```go
type Health struct {
    Connected    bool
    LastRevision uint64
    LastError    error
    LastSyncTime time.Time
}
```

Esse estado e util para observabilidade da sincronizacao sem afetar o hot path de avaliacao.

## Estrutura dos pacotes

### `types`

Contem os tipos de dominio usados pelo SDK, incluindo representacao de flag booleana e snapshot local.

### `cache`

Responsavel por manter o snapshot ativo em memoria, aplicar mutacoes e expor leitura local da revisao e das flags.

### `stream`

Encapsula o acesso gRPC ao `SDKService`, o loop de sincronizacao e a politica de reconnect.

### `evaluation`

Implementa a leitura booleana local em O(1) a partir do snapshot mantido em memoria.

### `client`

Expoe a API publica e coordena bootstrap, lifecycle do stream, estado de saude e shutdown.

## Comportamento atual no MVP

O comportamento atual do cliente segue estas propriedades:

- bootstrap assincrono: o cliente inicia com cache vazio e carrega o primeiro snapshot no loop em background;
- avaliacao local continua disponivel durante indisponibilidade de rede;
- reconnect nao apaga o ultimo snapshot valido;
- a ausencia de targeting avancado significa que a avaliacao e direta por chave;
- a API publica atual e pequena e focada em flags booleanas.

Exemplo minimo de uso:

```go
sdk, err := client.NewClient(ctx, client.Config{
    EnvAPIKey: "psk_...",
    Addr:      "localhost:50051",
})
if err != nil {
    return err
}
defer sdk.Close()

enabled := sdk.Enabled("checkout-redesign")
revision := sdk.Revision()
health := sdk.Health()

_, _, _ = enabled, revision, health
```

Um exemplo completo e executavel esta em `sdk/go/examples/simple/main.go`. Esse exemplo espera `500ms` antes da primeira leitura exatamente para dar tempo ao bootstrap inicial.

## O que falta

Itens ausentes hoje:

- testes de integracao do SDK contra servidor com Postgres + Redis reais;
- observabilidade mais profunda do loop de stream;
- benchmarks reais do SDK;
- expansao para tipos nao booleanos e cenarios de targeting.

Tambem faltam decisoes de produto e protocolo que impactam o SDK:

- semantica exata para flag inexistente na API publica;
- comportamento quando `enabled=false` e `bool_value=true`;
- estrategia explicita para revisao invalida ou atrasada;
- autenticacao/transporte alem do `env_api_key`;
- extensao futura para tipos nao booleanos.

## Criterios de evolucao

O repositorio ja atende ao nucleo do MVP do SDK. Para evolucoes futuras, um conjunto razoavel de criterios e:

- manter bootstrap de um ambiente via `GetSnapshot`;
- manter sincronizacao por `StreamUpdates`;
- avaliar flags booleanas localmente sem rede;
- reconectar sem perder o ultimo snapshot valido;
- ampliar cobertura com testes de integracao;
- publicar benchmarks reprodutiveis em `docs/benchmarks/benchmark.md`.
