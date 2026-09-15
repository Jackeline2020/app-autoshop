# ADR-003 — Uso de HPA (Horizontal Pod Autoscaler)

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Relacionado a** | [RFC-002 — Escolha da nuvem](../rfc/RFC-002-escolha-da-nuvem.md) |

## Contexto

A Fase 3 exige explicitamente um "Cluster Kubernetes com escalabilidade".
A AutoShop API é stateless (todo estado vive no RDS PostgreSQL), o que a
torna candidata natural a escalonamento horizontal — mas era preciso
decidir o mecanismo (HPA nativo do Kubernetes vs. escalonamento manual
vs. KEDA/escalonamento orientado a evento) e os parâmetros.

## Decisão

Usar o **HorizontalPodAutoscaler** nativo do Kubernetes
(`app-autoshop/k8s/base/hpa.yaml`), escalando o `Deployment autoshop-api`
por CPU e memória:

- Alvo: 70% de CPU / 80% de memória (média entre os pods).
- Mínimo de 2 réplicas (evita ponto único de falha mesmo em baixa carga).
- Máximo de 6 réplicas.

A métrica vem do `metrics-server` (instalado via Helm em todos os
ambientes — local, CI e AWS), que lê CPU/memória reais dos pods, sem
depender de métrica externa do New Relic para a decisão de escalonamento
em si (o New Relic só observa o resultado, não decide o scaling).

No nível de infraestrutura (não de pod), o **node group do EKS** também
escala (`node_min_size`/`node_max_size` no Terraform de
`infra-k8s-autoshop`) — o HPA escala pods primeiro; se os nodes existentes
não tiverem capacidade para os novos pods, o Cluster Autoscaler nativo do
EKS (Managed Node Group) provisiona mais instâncias EC2.

## Consequências

- Escalonamento reage a carga real de CPU/memória, sem intervenção manual
  — cobre o requisito de "escalabilidade" de forma mensurável e
  demonstrável em vídeo (gerar carga e observar o HPA reagir).
- Dois níveis de escala compostos (pods via HPA, nodes via node group)
  significam que picos muito súbitos podem esperar alguns minutos até o
  segundo nível (provisionamento de EC2) acompanhar o primeiro — aceitável
  para o padrão de tráfego esperado de uma oficina, não para um sistema de
  altíssima frequência.
- Não foi adotado KEDA (escalonamento orientado a evento/fila) — não há
  fila na arquitetura (ver [ADR-002](./ADR-002-padrao-comunicacao.md)), o
  que tornaria KEDA um mecanismo sem gatilho real para usar.
- `node_min_size = 2` mantém custo mínimo constante mesmo sem tráfego —
  contrapartida aceita conscientemente para garantir alta disponibilidade
  (2 AZs), reduzida ao mínimo viável para o orçamento da disciplina.
