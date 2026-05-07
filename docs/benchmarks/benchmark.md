# Benchmarks

## Objetivo

Os benchmarks do Pullsing devem validar se o desenho do servidor e do SDK atende as metas principais do projeto:

- avaliação local de flags com latência muito baixa e previsível;
- custo de atualização incremental menor que recarga completa de snapshot;
- comportamento estável sob reconexão e fanout de updates;
- ausência de rede no hot path de avaliação do SDK.

Este documento descreve metas, metodologia proposta e métricas alvo. Ele não apresenta números medidos porque, no estado atual do repositório, o SDK Go ainda não foi implementado e não existem benches executáveis publicados.

## Estado atual

Hoje o repositório fornece:

- contrato protobuf/gRPC em `proto/pullsing/v1/sdk.proto`;
- esqueleto do módulo `sdk/go`;
- nenhuma implementação do SDK;
- nenhum benchmark automatizado sob `sdk/go` ou `tests/load`.

Portanto, qualquer número neste momento seria fictício. O objetivo aqui é definir como medir quando houver código real.

## O que precisa ser medido

### 1. Avaliação local do SDK

Pergunta principal:

- qual é o custo de consultar uma flag booleana já carregada em memória?

Esse é o benchmark mais importante do SDK porque a proposta do produto depende de avaliação local em O(1) e sem rede.

Métricas desejadas:

- `ns/op`
- `B/op`
- `allocs/op`

Critério qualitativo:

- o caminho de leitura deve idealmente ter zero alocações por operação;
- a latência deve permanecer estável mesmo com grande número de flags carregadas.

### 2. Bootstrap por snapshot

Pergunta principal:

- quanto custa materializar um snapshot inicial em memória?

Esse benchmark mede:

- desserialização protobuf;
- construção do índice local por `key`;
- troca atômica do snapshot ativo.

Métricas desejadas:

- tempo total por bootstrap;
- memória alocada por snapshot;
- impacto do número de flags no tempo de inicialização.

### 3. Aplicação de updates incrementais

Pergunta principal:

- qual é o custo de aplicar `UPSERT` e `DELETE` no estado local?

Esse benchmark deve comparar:

- update pequeno sobre snapshot grande;
- lote de mutações;
- reconstrução total do snapshot versus aplicação incremental.

Métricas desejadas:

- `ns/op` ou `us/op` por mutação;
- bytes alocados por mutação ou por lote;
- tempo para publicar um novo snapshot imutável.

### 4. Reconnect e recuperação

Pergunta principal:

- qual é o custo operacional de perder o stream e se recuperar sem degradar avaliação local?

Esse cenário não é só de throughput. Ele precisa mostrar:

- tempo até reconexão;
- continuidade da avaliação usando o último snapshot válido;
- custo de novo snapshot quando houver divergência de revisão.

### 5. Servidor e fanout

Mesmo este workspace estando focado na documentação do SDK, o benchmark end-to-end também deve observar o lado do servidor quando ele existir:

- tempo entre persistir uma mudança e ela ficar disponível ao SDK;
- capacidade de fanout para múltiplos clientes via stream;
- impacto do Redis Pub/Sub no tempo de propagação.

## Metodologia proposta

### Ambiente de benchmark

Publicar sempre:

- versão do Go;
- commit exato do repositório;
- sistema operacional e arquitetura;
- CPU e quantidade de núcleos;
- configuração de `GOMAXPROCS`;
- se o benchmark rodou localmente ou em CI dedicada.

Sem isso, os números perdem comparabilidade.

### Ferramentas

Para SDK em Go, a base deve ser:

- `go test -bench=. -benchmem ./...`
- `go test -run=^$ -bench <nome> -count=10`

Quando houver comparação entre branches ou versões:

- `benchstat` para consolidar variação estatística.

Para cenários de carga integrados, usar ferramentas simples e reprodutíveis, com scripts versionados em `tests/load` ou `scripts/dev`, quando essas peças existirem.

### Perfis de carga propostos

### Perfil A: poucas flags

Uso para validar overhead fixo:

- 10 flags por ambiente;
- 1 mutação por update;
- 1 cliente.

### Perfil B: ambiente médio

Uso para cenário de aplicação comum:

- 1.000 flags por ambiente;
- 10 mutações por update;
- 10 a 100 clientes.

### Perfil C: ambiente grande

Uso para testar escala do índice local:

- 10.000 flags por ambiente;
- 100 mutações por update;
- leitura intensiva concorrente.

### Perfil D: churn alto

Uso para observar custo de atualização:

- frequência alta de updates;
- reconexões ocasionais;
- comparação entre custo incremental e custo de resnapshot.

## Regras para interpretação

- benchmark de avaliação deve ser separado de benchmark de rede;
- benchmark de stream deve separar custo de receber protobuf do custo de aplicar mutações;
- alocação por leitura é regressão importante;
- média sozinha não basta quando houver testes de latência integrada: sempre que possível registrar percentis;
- números devem vir acompanhados da forma de execução.

## Métricas alvo

As metas abaixo são de engenharia, não resultados já medidos.

### SDK local

- avaliação booleana em O(1);
- zero alocações no caminho quente de leitura;
- custo de leitura suficientemente baixo para não justificar qualquer cache externo adicional dentro do processo chamador.

### Atualização local

- aplicar mutações deve ser significativamente mais barato que reconstruir snapshot completo em cenários de update pequeno;
- reconnect não deve bloquear avaliação local já disponível.

### End-to-end

- propagação de mudança deve ser rápida o suficiente para uso de feature flags near-real-time;
- throughput do servidor deve crescer sem trabalho proporcional por avaliação do cliente, porque a avaliação acontece no SDK.

## Tabela de publicação sugerida

Quando os benchmarks existirem de fato, este documento deve passar a incluir tabelas como:

| Benchmark | Cenário | Métrica | Resultado |
| --- | --- | --- | --- |
| `BenchmarkBoolValueHit` | 1.000 flags, leitura single-thread | `ns/op` | `a medir` |
| `BenchmarkApplySingleUpsert` | snapshot 10.000 flags | `us/op` | `a medir` |
| `BenchmarkBootstrapSnapshot` | snapshot 10.000 flags | `ms/op` | `a medir` |
| `BenchmarkStreamReconnect` | reconnect com `since_revision` | tempo total | `a medir` |

## Próximos passos

Para transformar este plano em benchmark real, faltam:

- implementar o SDK Go;
- definir API pública mínima de leitura;
- criar benches em `sdk/go/...` com `testing.B`;
- criar cenário de integração com servidor, Postgres e Redis;
- publicar primeira rodada de números com comando exato e ambiente descrito.
