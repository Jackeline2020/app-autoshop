# ADR-002 — Padrão de comunicação: REST síncrono

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Relacionado a** | [RFC-001 — PostgreSQL](../rfc/RFC-001-postgres-database.md), [RFC-003 — Estratégia de autenticação](../rfc/RFC-003-estrategia-autenticacao.md) |

## Contexto

A Fase 3 introduz comunicação entre serviços que antes não existia: o
cliente HTTP fala com a Lambda de autenticação (via API Gateway) e,
separadamente, com a API principal (via Kubernetes Service). Era preciso
decidir se essa comunicação seria síncrona (REST/HTTP) ou assíncrona
(fila/evento), e se a API principal e a Lambda trocariam mensagens
diretamente entre si.

## Decisão

Manter **REST síncrono sobre HTTP/JSON** como único padrão de
comunicação externa, em todos os pontos de entrada:

- Cliente → API Gateway → Lambda (autenticação).
- Cliente → Service (LoadBalancer) → AutoShop API (CRUD, ordens de
  serviço).
- Provedor de e-mail → AutoShop API (webhook de atualização de status,
  mesma porta HTTP, autenticado por secret em vez de JWT).

A Lambda de autenticação **não chama a API principal em nenhum momento**
— ela consulta o PostgreSQL diretamente (mesmo banco, acesso via
credencial própria no Secrets Manager) e devolve o JWT direto ao cliente.
Isso evita uma dependência síncrona serviço-a-serviço que aumentaria a
superfície de falha do fluxo de login (se a API principal estivesse fora
do ar, o login continuaria funcionando).

## Consequências

- Nenhuma fila, broker de mensagens ou orquestrador de eventos foi
  introduzido — reduz a superfície de infraestrutura a provisionar e
  operar, compatível com o escopo e o prazo da disciplina.
- Contrapartida aceita: acoplamento direto ao schema do banco em dois
  lugares (API principal e Lambda) — mitigado porque as migrations
  (`app-autoshop/migrations`) são a única fonte de verdade do schema, e a
  Lambda só lê a tabela `customers` (superfície de leitura mínima e
  estável).
- Toda comunicação de rotas sensíveis é protegida por JWT (ver
  [RFC-003](../rfc/RFC-003-estrategia-autenticacao.md)); a comunicação
  específica do webhook de e-mail usa um secret próprio
  (`EmailWebhookMiddleware`) em vez de JWT, porque o provedor de e-mail
  não é um usuário autenticável da aplicação.
- Caminho de evolução, se o volume de OS crescesse muito: introduzir um
  evento assíncrono (`order_created`) publicado num broker, permitindo
  desacoplar consumidores futuros (ex: um serviço de notificação separado)
  sem reescrever o fluxo síncrono existente.
