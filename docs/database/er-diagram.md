# Modelo Entidade-Relacionamento — OS Service (PostgreSQL)

Schema do banco do OS Service, resultado das migrations em
[`migrations/`](../../migrations). Na Fase 4, as peças e o estoque passaram
para o Execution Service (DynamoDB): a OS guarda só uma cópia de nome,
preço e quantidade das peças usadas. As tabelas `saga_instances`, `outbox`
e `processed_messages` guardam o estado da Saga
([ADR-005](../adr/ADR-005-saga-orquestrada.md)).

```mermaid
erDiagram
    CUSTOMERS ||--o{ VEHICLES : possui
    CUSTOMERS ||--o{ ORDERS : solicita
    VEHICLES ||--o{ ORDERS : "é atendido em"
    ORDERS ||--o{ ORDER_SERVICES : contem
    ORDERS ||--o{ ORDER_PARTS : contem
    SERVICES ||--o{ ORDER_SERVICES : "referenciado por"
    ORDERS ||--o| SAGA_INSTANCES : "conduzida por"

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
        varchar status "ativo ou inativo"
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

    ORDERS {
        uuid id PK
        uuid customer_id FK
        varchar customer_name "cópia"
        uuid vehicle_id FK
        varchar vehicle_plate "cópia"
        order_status status
        numeric total_services
        numeric total_parts
        numeric total
        text notes
        text diagnosis
        text checkout_url
        text cancellation_reason
        timestamptz created_at
        timestamptz updated_at
        timestamptz diagnosis_at
        timestamptz waiting_approval_at
        timestamptz started_at
        timestamptz finished_at
        timestamptz delivered_at
    }

    ORDER_SERVICES {
        uuid id PK
        uuid order_id FK
        uuid service_id FK
        varchar service_name "cópia"
        numeric price "cópia"
        timestamptz executed_at
    }

    ORDER_PARTS {
        uuid id PK
        uuid order_id FK
        varchar part_id "ID no Execution"
        varchar part_name "cópia"
        int quantity
        numeric unit_price "cópia"
    }

    SAGA_INSTANCES {
        uuid id PK
        uuid order_id FK, UK
        varchar status
        varchar current_step
        text_array completed_steps
        text_array pending_compensations
        timestamptz deadline_at
        varchar correlation_id
        text failure_reason
        int version
        timestamptz created_at
        timestamptz updated_at
    }

    OUTBOX {
        uuid id PK
        varchar queue
        varchar message_type
        jsonb body
        timestamptz created_at
        timestamptz published_at
    }

    PROCESSED_MESSAGES {
        varchar message_id PK
        timestamptz processed_at
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

## Decisões de modelagem

- **Cliente, veículo e OS (1:N):** um cliente tem vários veículos e várias
  OS; cada OS tem um veículo. As chaves estrangeiras de `orders` usam
  `ON DELETE RESTRICT`: não é possível apagar cliente ou veículo com OS.
- **Endereço em colunas de `customers`:** o endereço é 1:1 com o cliente e
  não é compartilhado, então fica na própria tabela (`address_*`).
- **CPF ou CNPJ:** `CHECK (cpf IS NOT NULL OR cnpj IS NOT NULL)` e índices
  únicos parciais em cada documento.
- **Cópias na OS:** nome do cliente, placa, nome e preço de serviços e
  peças são gravados no momento da abertura. Uma mudança de preço ou de
  cadastro não altera OS antigas.
- **Peças sem chave estrangeira:** o catálogo de peças pertence ao
  Execution Service. `order_parts.part_id` guarda o ID da peça lá, sem
  referência a uma tabela local.
- **`order_status` como ENUM:** `recebida`, `em_diagnostico`,
  `aguardando_aprovacao`, `aguardando_pagamento`, `aguardando_execucao`,
  `em_execucao`, `finalizada`, `entregue`, `recusada`, `cancelada`. Um
  status inválido não é gravado.
- **Datas por status:** `diagnosis_at`, `waiting_approval_at`,
  `started_at`, `finished_at` e `delivered_at` são gravadas quando a OS
  entra em cada status e alimentam as métricas de tempo médio.
- **Saga na mesma base da OS:** a OS, a Saga e os comandos da `outbox` são
  gravados na mesma transação. `version` impede que duas decisões
  simultâneas sobre a mesma OS se sobreponham; `deadline_at` é o prazo do
  passo atual.
- **`processed_messages`:** IDs dos eventos já aplicados, para ignorar
  entregas repetidas do SQS.
