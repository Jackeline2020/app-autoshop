# RFC-001 — Adoção do PostgreSQL como banco de dados gerenciado

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Autores** | Jackeline Carvalho |
| **Relacionado a** | [ADR-002 — Padrão de comunicação REST](../adr/ADR-002-padrao-comunicacao.md), [`er-diagram.md`](../database/er-diagram.md) |

## Contexto

Nas Fases 1 e 2, o AutoShop usa DynamoDB (Local em dev/CI, opcionalmente DynamoDB real na AWS). A escolha fazia sentido para o MVP: schema flexível, custo por uso, baixa latência em acessos por chave (OS por ID, cliente por CPF).

A Fase 3 exige explicitamente um **Banco de Dados Gerenciado relacional**, com **modelagem documentada, garantindo consistência e performance**. Isso força a revisão da escolha original.

## Problema

O modelo de dados do AutoShop não é, na prática, um conjunto de itens independentes: uma Ordem de Serviço referencia obrigatoriamente um cliente, um veículo, e uma lista de serviços e peças — e o próprio domínio (`Order.Validate()`, `Order.CanTransitionTo()`) já implementa regras de integridade referencial e de máquina de estados **na aplicação**, porque o DynamoDB não tem como garantir isso no banco.

Consequências observadas no modelo atual:

- Nada impede, a nível de banco, criar uma OS apontando para um `customer_id` inexistente — a checagem é 100% responsabilidade do código Go.
- Consultas como "tempo médio de execução por status" (`GET /orders/metrics/average-time`, já existente) ou os futuros dashboards de observabilidade (volume diário de OS, erros por integração) exigem agregação — natural em SQL, contorcida em DynamoDB (scan + agregação em memória, ou GSIs dedicados por métrica).
- Duas gravações relacionadas (ex.: dar baixa no estoque de uma peça e criar a OS) não têm garantia atômica multi-item simples no DynamoDB sem usar transações explícitas mais verbosas.

## Decisão

Adotar **PostgreSQL**, provisionado como banco gerenciado (Amazon RDS for PostgreSQL) via Terraform, substituindo o DynamoDB.

Motivos:

1. **Consistência**: chaves estrangeiras, `CHECK` e `UNIQUE` constraints (ver [`er-diagram.md`](../database/er-diagram.md)) tiram do código Go a responsabilidade de garantir integridade referencial — o banco passa a rejeitar estados inválidos, não só a aplicação.
2. **Performance para o padrão de acesso real**: os endpoints de métricas e listagem por status (`GetAverageServiceTime`, `GetByCustomerID`, `GetStatus`) são consultas relacionais por natureza — índices B-tree em `orders.customer_id`, `orders.status` e `vehicles.plate` resolvem isso de forma direta, com `EXPLAIN ANALYZE` disponível para auditar o plano de execução (impossível de inspecionar da mesma forma no DynamoDB).
3. **Sem mudança de contrato**: como o domínio (`internal/domain`) e os casos de uso (`internal/usecase`) não conhecem DynamoDB nem Postgres — só falam com as interfaces de `internal/repository` — a migração fica isolada na camada de repositório e nas migrations SQL. Nenhuma regra de negócio, DTO ou teste de usecase muda.
4. **Driver, não ORM**: acesso via `pgx` + SQL explícito em vez de um ORM (GORM). Cada query fica visível e auditável — importante para justificar a modelagem no vídeo de entrega — e evita overhead de reflection/geração automática de SQL que dificultaria comprovar a performance pedida no enunciado.
5. **Gerenciado nativamente na AWS**: RDS for PostgreSQL cobre o requisito de "banco de dados gerenciado" sem sair do provedor de nuvem já usado para o cluster Kubernetes (EKS), simplificando rede (VPC compartilhada) e IAM.

## Consequências

- `internal/repository/*.go` passa a implementar as mesmas interfaces usadas hoje, mas com SQL via `pgx` em vez de chamadas ao SDK do DynamoDB.
- `pkg/config/dynamodb.go` é substituído por uma configuração de pool de conexão Postgres (`pgxpool`).
- Migrations versionadas (`golang-migrate`) tornam o schema auditável e reproduzível em qualquer ambiente (local, CI, AWS) — documentado no [ADR](../adr/) correspondente.
- `infra/aws/dynamodb.tf` é substituído por Terraform de RDS PostgreSQL; a política IAM de acesso a DynamoDB no IRSA (`infra/aws/irsa.tf`) dá lugar a credenciais via Secrets Manager + acesso de rede via Security Group.
- Se o volume de escrita crescer muito além do previsto, o RDS pode escalar verticalmente ou migrar para Aurora PostgreSQL (compatível, sem mudança de driver) — caminho de evolução preservado.
