# Fluxo da Ordem de Serviço (Saga)

Este documento mostra o caminho completo de uma OS, da abertura ao
pagamento e à finalização, indicando qual serviço é responsável por cada
etapa e como os serviços se comunicam. As decisões estão em
[ADR-005](adr/ADR-005-saga-orquestrada.md) (Saga orquestrada) e
[ADR-006](adr/ADR-006-mensageria-sqs.md) (SQS).

## Responsáveis por etapa

| Etapa | Responsável | Ação de quem usa o sistema |
|---|---|---|
| Abertura da OS | OS Service | Funcionário abre a OS |
| Diagnóstico | Execution | Mecânico registra o diagnóstico |
| Orçamento | Billing | — (gerado automaticamente) |
| Aprovação | OS Service | Cliente aprova ou recusa |
| Reserva de peças | Execution | — |
| Pagamento | Billing + Mercado Pago (ou mock) | Cliente paga |
| Execução | Execution | Mecânico inicia e finaliza o reparo |
| Entrega | OS Service | Funcionário registra a entrega |

## Caminho feliz

Setas contínuas são chamadas HTTP; setas tracejadas são mensagens SQS.

```mermaid
sequenceDiagram
    autonumber
    actor F as Funcionário / Mecânico
    actor C as Cliente
    participant OS as OS Service<br/>(orquestrador)
    participant EX as Execution
    participant BI as Billing
    participant MP as Mercado Pago / mock

    F->>OS: POST /orders
    OS->>EX: GET /parts/{id} (nome e preço)
    OS-->>F: 201 OS recebida
    OS--)EX: StartDiagnosis
    Note over OS: em_diagnostico
    F->>EX: Registra diagnóstico
    EX--)OS: DiagnosisFinished
    OS--)BI: GenerateBudget
    BI--)OS: BudgetGenerated
    Note over OS: aguardando_aprovacao
    C->>OS: PATCH /orders/{id}/approve
    OS--)EX: ReserveParts
    EX--)OS: PartsReserved
    Note over OS: aguardando_pagamento
    OS--)BI: RequestPayment
    BI->>MP: Token + criar pagamento
    MP-)BI: Webhook (pagamento atualizado)
    BI->>MP: Consulta status: approved
    BI--)OS: PaymentConfirmed
    OS--)EX: StartExecution
    EX--)OS: ExecutionQueued
    Note over OS: aguardando_execucao
    F->>EX: Inicia o reparo
    EX--)OS: ExecutionStarted
    Note over OS: em_execucao
    F->>EX: Finaliza o reparo
    EX--)OS: ExecutionFinished
    Note over OS: finalizada
```

## Compensação: pagamento recusado

As peças já estavam reservadas. Como o pagamento não aconteceu, a reserva
é desfeita e o orçamento cancelado.

```mermaid
sequenceDiagram
    participant OS as OS Service
    participant EX as Execution
    participant BI as Billing
    participant MP as Mercado Pago / mock

    OS--)BI: RequestPayment
    BI->>MP: Criar pagamento
    MP-)BI: Webhook
    BI->>MP: Consulta status: rejected
    BI--)OS: PaymentFailed
    Note over OS: Compensação em ordem inversa
    OS--)EX: ReleaseParts
    EX--)OS: PartsReleased
    OS--)BI: CancelBudget
    BI--)OS: BudgetCancelled
    Note over OS: cancelada
```

## Compensação: execução não inicia após o pagamento

É o cenário com mais passos a desfazer: pagamento, reserva e orçamento.

```mermaid
sequenceDiagram
    participant OS as OS Service
    participant EX as Execution
    participant BI as Billing

    OS--)EX: StartExecution
    Note over EX: Falha repetida (5 tentativas)
    Note over OS: ExecutionQueued não chega no prazo
    Note over OS: Compensação em ordem inversa
    OS--)BI: RefundPayment
    BI--)OS: PaymentRefunded
    OS--)EX: ReleaseParts
    EX--)OS: PartsReleased
    OS--)BI: CancelBudget
    BI--)OS: BudgetCancelled
    Note over OS: cancelada
```

## Todos os cenários de falha

| Falha | O que já foi feito em outro serviço | Compensação | Status final |
|---|---|---|---|
| Diagnóstico não conclui | Nada | Nenhuma | `cancelada` |
| Orçamento não é gerado | Diagnóstico (apenas registro) | Nenhuma | `cancelada` |
| Cliente recusa o orçamento | Orçamento | Cancelar orçamento | `recusada` |
| Estoque insuficiente | Orçamento | Cancelar orçamento | `cancelada` |
| Pagamento recusado ou expirado | Orçamento, reserva | Liberar peças, cancelar orçamento | `cancelada` |
| Execução não inicia | Orçamento, reserva, pagamento | Estornar, liberar peças, cancelar orçamento | `cancelada` |

Em todos os casos, falhas técnicas são retentadas antes (até 5 vezes) e
cada falha gera um evento no New Relic.

## Status da OS

```mermaid
stateDiagram-v2
    [*] --> recebida
    recebida --> em_diagnostico
    em_diagnostico --> aguardando_aprovacao
    em_diagnostico --> cancelada
    aguardando_aprovacao --> aguardando_pagamento: aprovada
    aguardando_aprovacao --> recusada: recusada
    aguardando_pagamento --> aguardando_execucao
    aguardando_pagamento --> cancelada
    aguardando_execucao --> em_execucao
    aguardando_execucao --> cancelada
    em_execucao --> finalizada
    finalizada --> entregue
    recusada --> [*]
    cancelada --> [*]
    entregue --> [*]
```
