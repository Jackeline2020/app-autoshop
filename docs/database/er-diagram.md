# Modelo Entidade-Relacionamento — PostgreSQL

Este diagrama descreve o schema relacional que substitui as tabelas DynamoDB da Fase 2. Ele deriva diretamente dos structs já existentes em `internal/domain/*.go` — nenhuma regra de negócio foi alterada, apenas a forma como os dados são persistidos.

```mermaid
erDiagram
    CUSTOMERS ||--o{ VEHICLES : possui
    CUSTOMERS ||--o{ ORDERS : solicita
    VEHICLES ||--o{ ORDERS : "é atendido em"
    ORDERS ||--o{ ORDER_SERVICES : contem
    ORDERS ||--o{ ORDER_PARTS : contem
    SERVICES ||--o{ ORDER_SERVICES : "referenciado por"
    PARTS ||--o{ ORDER_PARTS : "referenciado por"

    CUSTOMERS {
        uuid id PK
        varchar name
        varchar cpf UK "nullable"
        varchar cnpj UK "nullable"
        varchar email
        varchar phone
        varchar address_street
        varchar address_number
        varchar address_complement
        varchar address_city
        varchar address_state
        varchar address_zip_code
        timestamptz created_at
        timestamptz updated_at
    }

    VEHICLES {
        uuid id PK
        uuid customer_id FK
        varchar plate UK
        varchar brand
        varchar model
        int year
        timestamptz created_at
    }

    SERVICES {
        uuid id PK
        varchar name
        text description
        numeric price
        int estimated_time_minutes
        timestamptz created_at
    }

    PARTS {
        uuid id PK
        varchar name
        text description
        numeric price
        int stock
        int min_stock
        varchar unit
        timestamptz created_at
        timestamptz updated_at
    }

    ORDERS {
        uuid id PK
        uuid customer_id FK
        varchar customer_name "snapshot"
        uuid vehicle_id FK
        varchar vehicle_plate "snapshot"
        order_status status
        numeric total_services
        numeric total_parts
        numeric total
        text notes
        timestamptz created_at
        timestamptz updated_at
        timestamptz started_at
        timestamptz finished_at
        timestamptz delivered_at
    }

    ORDER_SERVICES {
        uuid id PK
        uuid order_id FK
        uuid service_id FK
        varchar service_name "snapshot"
        numeric price "snapshot"
        timestamptz executed_at
    }

    ORDER_PARTS {
        uuid id PK
        uuid order_id FK
        uuid part_id FK
        varchar part_name "snapshot"
        int quantity
        numeric unit_price "snapshot"
    }

    USERS {
        uuid id PK
        varchar name
        varchar email UK
        varchar password_hash
        varchar role
        timestamptz created_at
    }
```

## Relacionamentos e decisões de modelagem

**CUSTOMERS → VEHICLES (1:N)** e **CUSTOMERS → ORDERS (1:N)**: um cliente pode ter vários veículos e várias ordens de serviço. `vehicles.customer_id` e `orders.customer_id` são chaves estrangeiras `NOT NULL`, refletindo `Order.Validate()` que já exige `CustomerID` e `VehicleID`.

**VEHICLES → ORDERS (1:N)**: cada OS está associada a exatamente um veículo. `ON DELETE RESTRICT` em ambas as FKs de `orders` — não faz sentido apagar um cliente ou veículo que tenha OS associada; isso é uma regra de integridade que o DynamoDB não garantia (a aplicação tinha que checar isso na mão).

**Endereço embutido em `customers`**: `Address` no domínio é 1:1 e nunca reutilizado por outra entidade, então virou colunas prefixadas (`address_*`) na própria tabela em vez de uma tabela `addresses` separada — evita um JOIN desnecessário em toda consulta de cliente, sem perder normalização (não há redundância).

**`orders.customer_name` e `orders.vehicle_plate` também são snapshot**, pelo mesmo motivo do parágrafo abaixo: a OS mantém o nome do cliente e a placa do veículo tal como estavam no momento da abertura, sem depender de um JOIN nem mudar se o cadastro for editado depois.

**`ORDER_SERVICES` e `ORDER_PARTS` guardam snapshot de nome e preço**: isso é proposital, não um erro de normalização. `OrderService.Price` e `OrderPart.UnitPrice` no domínio já capturam o valor no momento da OS — se o preço de um serviço mudar depois, uma OS antiga não pode ser recalculada retroativamente. As FKs (`service_id`, `part_id`) continuam existindo para rastreabilidade, mas o valor histórico vem das colunas snapshot, não de um JOIN com `services`/`parts`.

**`order_status` como ENUM nativo do Postgres** (`recebida`, `em_diagnostico`, `aguardando_aprovacao`, `em_execucao`, `finalizada`, `entregue`, `recusada`): substitui a validação apenas em memória da máquina de estados (`Order.CanTransitionTo`) por uma restrição também no banco — um valor de status inválido não consegue nem ser gravado.

**`cpf` e `cnpj` com `UNIQUE` parcial (`WHERE cpf IS NOT NULL`)**: no domínio, `Customer.Validate()` exige CPF *ou* CNPJ, nunca os dois vazios. O banco reforça isso com um `CHECK (cpf IS NOT NULL OR cnpj IS NOT NULL)` e índices únicos parciais, algo que o DynamoDB não expressa nativamente sem lógica extra na aplicação.

**Chaves primárias `UUID`**: mantém o mesmo formato de ID que já era usado no DynamoDB (string), evitando alterar contratos de API (`internal/dto/*.go`) — os IDs continuam opacos para quem consome a API.
