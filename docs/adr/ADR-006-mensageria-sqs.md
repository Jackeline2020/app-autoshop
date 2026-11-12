# ADR-006 — Mensageria com Amazon SQS

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 4 |
| **Substitui** | [ADR-002 — Padrão de comunicação: REST síncrono](ADR-002-padrao-comunicacao.md), na comunicação entre serviços |
| **Relacionado a** | [RFC-005 — Divisão em microsserviços](../rfc/RFC-005-microsservicos.md), [ADR-005 — Saga orquestrada](ADR-005-saga-orquestrada.md) |

## Contexto

Os passos da Saga envolvem três serviços que podem ficar fora do ar ou
lentos de forma independente. Se o orquestrador chamasse os serviços por
REST, uma indisponibilidade momentânea derrubaria o fluxo inteiro. É
preciso um canal que guarde as mensagens até o destino conseguir
processá-las.

## Decisão

Usar o **Amazon SQS** para toda a comunicação da Saga.

### Filas

| Fila | Quem publica | Quem consome | Conteúdo |
|---|---|---|---|
| `execution-commands` | OS Service | Execution | `StartDiagnosis`, `ReserveParts`, `ReleaseParts`, `StartExecution` |
| `billing-commands` | OS Service | Billing | `GenerateBudget`, `CancelBudget`, `RequestPayment`, `RefundPayment` |
| `os-events` | Billing e Execution | OS Service | Resultado de cada comando (sucesso ou falha) |

Cada fila tem uma **DLQ** (fila de mensagens mortas) associada.

### Formato das mensagens

Todas as mensagens seguem o mesmo envelope em JSON:

```json
{
  "message_id": "uuid",
  "type": "ReserveParts",
  "saga_id": "uuid",
  "order_id": "uuid",
  "correlation_id": "uuid",
  "occurred_at": "2026-11-11T10:00:00Z",
  "payload": { }
}
```

O `correlation_id` liga logs e traces da mesma OS em todos os serviços.

### Retentativa e falhas

- Uma mensagem só é apagada da fila depois de processada com sucesso. Se
  o consumidor falhar ou cair, ela volta para a fila após o tempo de
  visibilidade (30 s) e é processada de novo.
- Depois de **5 tentativas**, a mensagem vai para a DLQ, e o
  orquestrador trata o passo como falho (ADR-005).
- Além disso, o orquestrador tem um **prazo por passo**: se a resposta
  não chegar no tempo esperado, o passo também é considerado falho.

### Idempotência

O SQS garante que a mensagem é entregue **pelo menos uma vez**, então a
mesma mensagem pode chegar duas vezes. Cada consumidor registra os
`message_id` já processados e ignora repetições. Comandos como
`ReleaseParts` e `RefundPayment` também verificam o estado atual antes de
agir (não liberam peças já liberadas nem estornam pagamento já
estornado).

### Ambientes

| Ambiente | SQS |
|---|---|
| Local e homologação | LocalStack (mesma API da AWS) |
| Produção | Amazon SQS, provisionado por Terraform no repositório de cada serviço consumidor |

O código é o mesmo nos dois casos; muda apenas o endpoint configurado.

## Alternativas consideradas

| Opção | Motivo do descarte |
|---|---|
| **REST síncrono entre serviços** | Indisponibilidade de um serviço interromperia a Saga; não guarda a mensagem para depois |
| **RabbitMQ** | Exigiria operar e monitorar um broker dentro do cluster |
| **Kafka** | Mais complexo do que o volume e o escopo exigem |
| **SQS FIFO** | Não é necessário: o orquestrador envia um comando por vez para cada Saga, então a ordem já é garantida |

## Consequências

- Um serviço fora do ar não perde mensagens: elas aguardam na fila até
  ele voltar.
- Mensagens que falham repetidamente ficam isoladas na DLQ, visíveis
  para análise, sem travar as demais.
- Todos os consumidores precisam ser idempotentes.
- Por ser um serviço gerenciado da AWS, não há broker para manter no
  cluster.
