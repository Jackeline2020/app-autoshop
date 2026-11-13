# ADR-005 — Saga orquestrada

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 4 |
| **Relacionado a** | [RFC-005 — Divisão em microsserviços](../rfc/RFC-005-microsservicos.md), ADR-006 (Mensageria com SQS) |

## Contexto

O fluxo de uma OS passa pelos três serviços: o Execution faz o
diagnóstico e reserva as peças, o Billing gera o orçamento e cobra, e o
OS Service registra o andamento. Cada serviço grava no próprio banco, então
não existe uma transação única que desfaça tudo em caso de falha. É
preciso coordenar os passos e, quando um deles falha, desfazer o que os
anteriores já fizeram em outros serviços.

## Decisão

Usar o **Saga Pattern na forma orquestrada**, com o orquestrador dentro
do **OS Service**.

- O orquestrador envia **comandos** para os serviços e recebe **eventos**
  com o resultado. Os serviços não conversam entre si.
- O estado de cada Saga fica na tabela `saga_instances`: passo atual,
  passos concluídos e número de tentativas. A Saga e a OS são atualizadas
  na mesma transação.
- Os comandos são gravados numa tabela **outbox** na mesma transação e
  publicados depois, para que nenhum comando se perca se o serviço cair
  entre gravar e publicar.

### Passos

| # | Comando | Serviço | Evento de resposta | Status da OS durante o passo |
|---|---|---|---|---|
| 1 | `StartDiagnosis` | Execution | `DiagnosisFinished` | `em_diagnostico` |
| 2 | `GenerateBudget` | Billing | `BudgetGenerated` | `em_diagnostico` |
| 3 | *(cliente aprova ou recusa)* | OS | — | `aguardando_aprovacao` |
| 4 | `ReserveParts` | Execution | `PartsReserved` | `aguardando_pagamento` |
| 5 | `RequestPayment` | Billing | `PaymentConfirmed` | `aguardando_pagamento` |
| 6 | `StartExecution` | Execution | `ExecutionQueued` | `aguardando_execucao` |
| 7 | *(mecânico inicia e finaliza o reparo)* | Execution | `ExecutionStarted`, depois `ExecutionFinished` | `em_execucao`, depois `finalizada` |

O passo 6 confirma apenas que a OS entrou na fila de reparo. O início e o
fim do reparo dependem do mecânico e podem levar horas, por isso chegam
como eventos separados e não contam no prazo do passo.

Os status `aguardando_pagamento`, `aguardando_execucao` e `cancelada` são
novos; os demais são os mesmos das fases anteriores.

### Falhas e compensações

Antes de qualquer compensação, falhas técnicas (serviço fora do ar,
timeout) são **retentadas** (ADR-006). Só quando as tentativas se esgotam,
ou quando a falha é de negócio, a Saga decide:

- se **nenhum passo anterior precisa ser desfeito**, a falha é apenas
  tratada: a OS é cancelada e um evento de falha é registrado;
- se **algum passo anterior alterou dados em outro serviço**, esses
  passos são **compensados**, em ordem inversa.

| Falha | Compensação | Status final |
|---|---|---|
| Diagnóstico ou orçamento não concluem | Nenhuma (nada a desfazer) | `cancelada` |
| Cliente recusa o orçamento | Cancelar orçamento | `recusada` |
| Estoque insuficiente | Cancelar orçamento | `cancelada` |
| Pagamento recusado ou expirado | Liberar peças, cancelar orçamento | `cancelada` |
| Execução não inicia após o pagamento | Estornar pagamento, liberar peças, cancelar orçamento | `cancelada` |

As peças são reservadas **antes** da cobrança. Assim, na maior parte das
falhas a compensação é só devolver o estoque, sem precisar estornar
pagamento.

## Alternativas consideradas

**Saga coreografada**, em que cada serviço reage aos eventos dos outros
sem coordenador central. Descartada porque:

- o fluxo tem seis passos e cinco cenários de falha; espalhados entre
  três serviços, ficariam difíceis de entender e de testar;
- não haveria um lugar único para consultar em que passo está cada OS;
- cada serviço precisaria conhecer os eventos dos outros, aumentando o
  acoplamento.

## Consequências

- O fluxo inteiro e todas as compensações estão em um só lugar, fácil de
  ler, testar (BDD) e acompanhar no New Relic.
- Billing e Execution só conhecem os próprios comandos e não dependem um
  do outro.
- O OS Service concentra a coordenação. Se ele ficar fora do ar, as
  Sagas param e retomam do último passo registrado quando ele voltar,
  porque estado e comandos pendentes estão persistidos.
- Todos os comandos, inclusive os de compensação, precisam ser
  idempotentes, já que podem ser entregues mais de uma vez.
