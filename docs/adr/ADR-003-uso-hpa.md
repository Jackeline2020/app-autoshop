# ADR-003 — Uso de HPA (Horizontal Pod Autoscaler)

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Relacionado a** | [RFC-002 — Escolha da nuvem](../rfc/RFC-002-escolha-da-nuvem.md) |

## Contexto

A Fase 3 pede um cluster Kubernetes com escalabilidade. O OS Service
(`autoshop-api`) não guarda estado no pod (os dados ficam no PostgreSQL),
então pode ter várias réplicas atendendo ao mesmo tempo.

## Decisão

Usar o **HorizontalPodAutoscaler** nativo do Kubernetes
(`k8s/base/hpa.yaml`), escalando o Deployment `autoshop-api` por CPU e
memória:

- alvo: 70% de CPU e 80% de memória (média entre os pods);
- mínimo de 2 réplicas, para não haver ponto único de falha;
- máximo de 6 réplicas.

O HPA lê CPU e memória dos pods pelo `metrics-server`, instalado por Helm
no cluster.

## Consequências

- O número de réplicas acompanha a carga, sem intervenção manual.
- O HPA escala pods, não nodes. O cluster tem 3 nodes fixos
  (`node_desired_size` no Terraform do `infra-k8s-autoshop`); mais nodes
  exigem alterar esse valor.
- Na Fase 4, o HPA continua só no OS Service. Execution e Billing rodam
  com uma réplica cada.
