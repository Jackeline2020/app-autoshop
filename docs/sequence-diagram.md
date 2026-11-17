# Diagrama de Sequência

Cobre a autenticação via CPF e a abertura de uma ordem de serviço usando o
token emitido nesse fluxo. O que acontece depois da abertura (diagnóstico,
orçamento, pagamento e execução) está em [`saga-flow.md`](./saga-flow.md).

## 1. Autenticação via CPF

O cliente nunca fala diretamente com o banco nem com a lambda de dentro da
aplicação principal — o fluxo de autenticação é isolado num repositório e
numa Function Serverless própria (`lambda-auth-autoshop`), atrás do API
Gateway. A regra de negócio (validar CPF, checar existência e status do
cliente, emitir o JWT) vive em `internal/authflow`, reaproveitada tanto pelo
handler real da Lambda quanto pelo entrypoint de desenvolvimento local
(`cmd/local`), então o comportamento é idêntico em produção e localmente.

```mermaid
sequenceDiagram
    actor Cliente
    participant APIGW as API Gateway
    participant Lambda as Lambda auth<br/>(internal/authflow)
    participant OS as OS Service<br/>(rota interna)

    Cliente->>APIGW: POST /auth { cpf }
    APIGW->>Lambda: invoke (proxy integration)
    Lambda->>Lambda: valida formato do CPF

    alt CPF mal formado
        Lambda-->>APIGW: 400 Bad Request
        APIGW-->>Cliente: 400 Bad Request
    else CPF válido
        Lambda->>OS: GET /internal/customers/by-cpf/{cpf}<br/>(token de serviço)
        OS-->>Lambda: cliente (ou 404)

        alt cliente não encontrado
            Lambda-->>APIGW: 404 Not Found
            APIGW-->>Cliente: 404 Not Found
        else cliente inativo
            Lambda-->>APIGW: 403 Forbidden
            APIGW-->>Cliente: 403 Forbidden ("cliente inativo")
        else cliente ativo
            Lambda->>Lambda: gera JWT (HS256, claims: sub=customer_id, role=customer)
            Lambda-->>APIGW: 200 { token }
            APIGW-->>Cliente: 200 { token }
        end
    end
```

## 2. Abertura de uma ordem de serviço

O token do fluxo acima (ou o token de funcionário, emitido por
`/auth/login`) é usado como `Bearer` na chamada à API principal, que roda
nos pods do EKS. A validação de negócio (cliente, veículo, serviços e
peças, consultadas no Execution) acontece em sequência no `OrderUseCase.Create`, e cada
etapa — sucesso ou falha — gera um evento de observabilidade (New Relic +
log estruturado), usado pelos dashboards e pelo alerta de falha em OS.

```mermaid
sequenceDiagram
    actor Cliente
    participant API as AutoShop API<br/>(AuthMiddleware + OrderHandler)
    participant UC as OrderUseCase
    participant DB as PostgreSQL do OS
    participant EX as Execution Service
    participant NR as New Relic

    Cliente->>API: POST /orders<br/>Authorization: Bearer {token}<br/>{ customer_id, vehicle_id, services[], parts[] }
    API->>API: AuthMiddleware valida o JWT

    alt token inválido/ausente
        API-->>Cliente: 401 Unauthorized
    else token válido
        API->>UC: Create(customerId, vehicleId, services, parts, correlationId)

        UC->>DB: FindByID(customer)
        alt cliente não encontrado
            UC->>NR: RecordOrderFailure("create_validate_customer")
            UC-->>API: erro
            API-->>Cliente: 422 Unprocessable Entity
        else cliente ok
            UC->>DB: FindByID(vehicle)
            alt veículo não encontrado
                UC->>NR: RecordOrderFailure("create_validate_vehicle")
                UC-->>API: erro
                API-->>Cliente: 422 Unprocessable Entity
            else veículo ok
                UC->>DB: FindByID(service) por item
                UC->>EX: GET /parts/{id} por item (nome e preço)
                alt serviço ou peça não encontrado
                    UC->>NR: RecordOrderFailure("create_validate_service/part")
                    UC-->>API: erro
                    API-->>Cliente: 422 Unprocessable Entity
                else tudo válido
                    UC->>DB: grava OS + saga_instances + outbox (mesma transação)
                    UC->>NR: RecordOrderEvent("order_created")
                    UC-->>API: OS criada
                    API-->>Cliente: 201 Created { order }
                end
            end
        end
    end
```
