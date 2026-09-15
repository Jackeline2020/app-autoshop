# ADR-004 — Logs estruturados (JSON) com correlação entre requisições

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Relacionado a** | [RFC-004 — Observabilidade](../rfc/RFC-004-observabilidade.md) |

## Contexto

Até a Fase 2, a AutoShop API usava o logger padrão do Gin (texto livre,
sem estrutura, sem correlação entre linhas de log da mesma requisição). A
Fase 3 exige explicitamente "logs estruturados (JSON), incluindo
correlação entre requisições".

## Decisão

Substituir o logger padrão do Gin por um logger estruturado próprio
(`internal/middleware/logging.go`), baseado em `log/slog` (biblioteca
padrão do Go, sem dependência externa), com as seguintes regras:

- Todo log é emitido em JSON, com campos fixos: `correlation_id`,
  `method`, `path`, `status`, `latency`, e — quando o agente New Relic
  está ativo — `trace_id`/`span_id` da transação corrente.
- O `correlation_id` é lido do header de entrada `X-Correlation-ID`, se o
  cliente já enviou um (permite rastrear uma requisição através de
  múltiplos serviços, ex: um front-end que já gera esse header); se
  ausente, o middleware gera um novo (UUID). O valor é sempre devolvido no
  header de resposta, para o cliente poder correlacionar do outro lado.
- O nível do log segue o status HTTP: `INFO` para 2xx/3xx, `WARN` para
  4xx (erro de cliente/validação), `ERROR` para 5xx — sem exigir que cada
  handler decida o nível manualmente.
- Eventos de negócio (`pkg/observability/events.go`) usam o mesmo
  `correlation_id`, propagado explicitamente pelos handlers
  (`middleware.CorrelationID(c)`) até o usecase — permite achar, num único
  filtro de log ou consulta NRQL, todas as linhas relacionadas a uma
  requisição específica, incluindo o evento de negócio que ela gerou.

## Consequências

- Nenhuma dependência externa nova para logging (`log/slog` é biblioteca
  padrão desde Go 1.21) — mantém a árvore de dependências do `go.mod`
  enxuta.
- `correlation_id` funciona mesmo sem o New Relic configurado (não
  depende de `trace_id`/`span_id`, que só existem com o agente ativo) —
  os logs continuam correlacionáveis em ambiente local/CI sem conta no
  New Relic.
- Nível de log automático por status HTTP significa que uma falha de
  validação de negócio (422, ex: "cliente não encontrado" na criação de
  OS) aparece como `WARN` no log de requisição — isso é esperado e
  correto, e é um log **diferente** do evento `ERROR`
  `order_processing_failure` emitido separadamente por
  `RecordOrderFailure` (que é o que de fato alimenta o alerta de falha em
  OS). As duas coisas não devem ser confundidas ao ler o log.
- Contrapartida aceita: como o `correlation_id` é gerado por requisição
  HTTP (não por processo de negócio de ponta a ponta), um fluxo que
  atravessa múltiplas chamadas do cliente (ex: criar OS, depois consultar
  status em outra requisição) tem `correlation_id` diferente em cada
  etapa — rastreável pelo `order_id`, não por um único `correlation_id`
  compartilhado entre chamadas distintas.
