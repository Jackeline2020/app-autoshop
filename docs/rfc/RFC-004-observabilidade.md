# RFC-004 — Ferramenta e estratégia de observabilidade

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Autores** | Jackeline Carvalho |
| **Relacionado a** | [ADR-004 — Logs estruturados e traces](../adr/ADR-004-logs-estruturados-traces.md) |

## Contexto

A Fase 3 exige monitorar latência das APIs, consumo de CPU/memória do
Kubernetes, healthchecks/uptime, alertas de falha no processamento de OS,
e logs estruturados com correlação entre requisições — expostos em
dashboards de volume diário de OS, tempo médio de execução por status, e
erros/falhas nas integrações. O enunciado permite escolha livre entre
Datadog e New Relic.

## Problema

Nenhuma das duas ferramentas estava em uso no projeto antes da Fase 3, e
o requisito cobre três domínios diferentes ao mesmo tempo — métricas de
infraestrutura (Kubernetes), APM (latência/traces da API) e eventos de
negócio (volume de OS, falhas de processamento) — que em algumas
ferramentas exigem produtos/planos separados.

## Decisão

Adotar o **New Relic**, no tier gratuito ("New Relic Free"), integrado em
três frentes:

1. **APM da aplicação** — `github.com/newrelic/go-agent/v3` +
   `nrgin.Middleware`, inicializado em `cmd/api/main.go`
   (`pkg/observability/newrelic.go`). Cobre latência por rota, traces
   distribuídos e captura automática de erro sem instrumentação manual em
   cada handler.
2. **Infraestrutura Kubernetes** — Helm chart `nri-bundle` (New Relic
   Infrastructure), instalado como DaemonSet no cluster (local via `kind`
   e, depois do deploy, no EKS real) — cobre CPU/memória dos nodes e dos
   pods sem depender de código na aplicação.
3. **Eventos de negócio** — `pkg/observability/events.go`
   (`RecordOrderEvent`/`RecordOrderFailure`), instrumentando
   `OrderUseCase` em cada transição relevante (criação de OS, mudança de
   status, aprovação/recusa de orçamento, falha de validação/estoque).
   Cada chamada grava simultaneamente um evento customizado no New Relic
   (`OrderLifecycle`/`OrderProcessingFailure`) **e** um log estruturado
   (`log/slog`, formato JSON), então a mesma instrumentação alimenta
   dashboard, alerta e log ao mesmo tempo, sem três implementações
   separadas.

Motivos da escolha, frente a Datadog:

- **Tier gratuito permanente** (sem cartão de crédito e sem expiração de
  trial) — Datadog exige cartão mesmo no free tier e tem um trial com
  prazo, incompatível com o cronograma da disciplina.
- **Um agente cobre APM + eventos customizados**, sem precisar combinar
  produtos (ex: Datadog APM + Datadog Log Management como contratos
  distintos).
- **NRQL** (a linguagem de consulta do New Relic) é próxima o bastante de
  SQL pra consultar os eventos de negócio diretamente
  (`SELECT count(*) FROM OrderProcessingFailure ...`), sem precisar de
  um pipeline de agregação externo.

Detalhe de implementação que vale registrar: diferente de outros agentes
New Relic (Node, Python, Java), o agente Go **não lê** `NEW_RELIC_LOG`/
`NEW_RELIC_LOG_LEVEL` como variável de ambiente — o log de diagnóstico do
próprio agente só liga via `newrelic.ConfigDebugLogger` no código,
condicionado aqui a uma flag própria (`NEW_RELIC_DEBUG=true`), mantida
desligada por padrão.

## Dashboards e alerta configurados

- **Total de OS criadas** / **Total de OS criadas por período** (facetado
  por tipo de evento) — cobre "volume diário de ordens de serviço".
- **Tempo médio de execução por status** (Diagnóstico, Execução,
  Finalização) — `SELECT average(durationMinutes) FROM OrderLifecycle
  WHERE eventName = 'status_duration' FACET status`, calculado a partir
  dos timestamps de transição gravados em `domain.Order`.
- **Falhas na última hora** / **Total de falhas na OS por período** /
  **Tipos de Erros** — cobre "erros e falhas nas integrações" e alimenta o
  alert condition.
- **Latência** (avg/p95) — automático via `nrgin`, cobre "latência das
  APIs".
- **Alert condition** NRQL (Critical, `SELECT count(*) FROM
  OrderProcessingFailure`, `above 0 por pelo menos 1 minuto`) + workflow de
  notificação por e-mail — cobre "alertas para falhas no processamento de
  ordens de serviço", testado de ponta a ponta.

## Consequências

- A aplicação roda normalmente sem `NEW_RELIC_LICENSE_KEY` configurada
  (observabilidade fica desligada, sem quebrar testes/CI/dev local) — a
  chave nunca é uma dependência obrigatória para a API subir.
- O mesmo `NEW_RELIC_APP_NAME` precisa ser mantido entre o ambiente local
  e a AWS pra que os dados dos dois ambientes se somem no mesmo conjunto
  de dashboards, em vez de criar uma entidade nova no New Relic.
- Contrapartida aceita: tier gratuito tem retenção de dados limitada (não
  é histórico permanente) — aceitável para o escopo da disciplina.
- "Healthchecks e uptime" cobrem a checagem interna (`/health` + `nrgin`);
  o Synthetic Monitor de uptime externo só é possível depois do deploy na
  AWS, porque exige uma URL pública.
