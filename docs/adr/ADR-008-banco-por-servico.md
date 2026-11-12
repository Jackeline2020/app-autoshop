# ADR-008 — Banco de dados por serviço

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 4 |
| **Relacionado a** | [RFC-001 — PostgreSQL](../rfc/RFC-001-postgres-database.md), [RFC-005 — Divisão em microsserviços](../rfc/RFC-005-microsservicos.md) |

## Contexto

Cada microsserviço precisa ter o próprio banco, e nenhum serviço pode
acessar o banco de outro. A solução deve usar ao menos um banco
relacional e um não relacional. O deploy da infraestrutura de cada banco
também precisa ser independente dos demais.

## Decisão

| Serviço | Banco | Dados |
|---|---|---|
| **OS Service** | PostgreSQL (instância RDS própria) | Clientes, veículos, catálogo de serviços, OS, histórico de status, estado da Saga, outbox |
| **Billing Service** | PostgreSQL (instância RDS própria) | Orçamentos, itens do orçamento, pagamentos, estornos |
| **Execution Service** | DynamoDB | Peças e estoque, reservas, diagnósticos, fila de execução |

### Por que PostgreSQL no OS e no Billing

- **OS:** os dados são relacionais (cliente → veículo → OS → itens), e
  a Saga precisa gravar o estado da OS, o estado da Saga e a outbox **na
  mesma transação**.
- **Billing:** lida com valores financeiros, que exigem transações e
  restrições de unicidade (por exemplo, um único pagamento aprovado por
  orçamento).

### Por que DynamoDB no Execution

- O acesso é sempre por chave (peça por `part_id`, reserva e execução
  por `order_id`), sem necessidade de junções.
- A reserva de peças usa **escrita condicional** (`estoque >= quantidade`)
  em uma única **transação** (`TransactWriteItems`): ou todas as peças da
  OS são reservadas, ou nenhuma. Isso evita vender a mesma peça duas
  vezes quando várias OS são processadas ao mesmo tempo.
- É um serviço gerenciado e escala automaticamente com o volume das
  filiais, sem servidor para dimensionar.

| Tabela | Chave | Conteúdo |
|---|---|---|
| `parts` | `part_id` | Nome, preço, estoque, estoque mínimo |
| `reservations` | `order_id` | Peças e quantidades reservadas, status (reservada, liberada, consumida) |
| `executions` | `order_id` | Diagnóstico, status da execução, datas de início e fim |

### Isolamento e provisionamento

- Cada serviço tem **credenciais próprias**, armazenadas no Secrets
  Manager e entregues apenas ao seu pod. Um serviço não tem como se
  conectar ao banco de outro.
- O repositório `infra-db-autoshop` tem **um Terraform independente por
  banco** (`os/`, `billing/`, `execution/`), cada um com o próprio estado
  e o próprio job no pipeline. Criar, alterar ou destruir um banco não
  afeta os outros.
- Cada serviço aplica as próprias migrações (PostgreSQL) ou cria as
  próprias tabelas (DynamoDB).

### Ambientes

| Ambiente | PostgreSQL | DynamoDB |
|---|---|---|
| Local e homologação | Um container por serviço | LocalStack |
| Produção | Duas instâncias Amazon RDS | Amazon DynamoDB |

## Alternativas consideradas

| Opção | Motivo do descarte |
|---|---|
| **Uma instância RDS com um database por serviço** | Mais barata, mas o banco do Billing passaria a depender da instância do OS para existir, e o deploy deixaria de ser independente |
| **MongoDB / DocumentDB no Execution** | Exigiria operar um servidor ou pagar uma instância fixa; o DynamoDB atende os padrões de acesso sem servidor |

## Consequências

- Cada serviço evolui o próprio modelo de dados sem coordenar com os
  outros.
- Não há consultas que cruzem bancos. Dados de outro serviço chegam por
  mensagem (cópia) ou por API (RFC-005).
- Duas instâncias RDS aumentam o custo da infraestrutura enquanto ela
  estiver no ar.
