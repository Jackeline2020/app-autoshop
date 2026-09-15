# RFC-003 — Estratégia de autenticação (CPF + JWT via Function Serverless)

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Autores** | Jackeline Carvalho |
| **Relacionado a** | [RFC-002 — Escolha da nuvem](./RFC-002-escolha-da-nuvem.md), [`docs/sequence-diagram.md`](../sequence-diagram.md) |

## Contexto

Nas Fases 1/2, a AutoShop API tinha um único mecanismo de login: usuário e
senha fixos de funcionário (`/auth/login`, ver `internal/handler/auth_handler.go`),
gerando um JWT simples. A Fase 3 exige um segundo mecanismo, específico
para clientes: autenticação via CPF, com uma Function Serverless
responsável por validar o CPF, consultar existência **e status** do
cliente, e emitir o token.

## Problema

Dois requisitos aparentemente conflitantes: (1) a autenticação por CPF
precisa ser uma Function Serverless separada, rodando fora da aplicação
principal; (2) o token gerado por ela precisa ser aceito pelas mesmas
rotas protegidas da API principal (ex: `GET /orders/customer/:customer_id`),
sem duplicar lógica de validação de JWT nem criar dois formatos de token
incompatíveis entre si.

Havia ainda uma lacuna de segurança real encontrada durante a Fase 3: a
rota `GET /orders/customer/:customer_id` estava completamente pública —
qualquer pessoa sabendo (ou adivinhando) um `customer_id` via a lista de
OS de qualquer cliente, sem token nenhum. Isso não validava de fato o
propósito do fluxo de CPF (impedir que um cliente veja OS de outro).

## Decisão

1. **Repositório dedicado** (`lambda-auth-autoshop`) para a Function
   Serverless, atrás de um API Gateway HTTP API — cobre o requisito de
   repositório separado com CI/CD próprio.
2. **Mesmo formato de JWT** (`pkg/auth`, HS256) usado tanto pelo login de
   funcionário (`/auth/login`, role `admin`) quanto pela Lambda de CPF
   (role `customer`) — os dois processos compartilham o mesmo
   `JWT_SECRET`, então a API principal valida ambos com o mesmo
   middleware (`AuthMiddleware`), sem branch de código por origem do
   token.
3. **Regra de negócio isolada e reutilizável**: a lógica de "validar CPF →
   buscar cliente → checar status → emitir token" vive em
   `lambda-auth-autoshop/internal/authflow`, chamada tanto pelo handler
   real da Lambda (`main.go`, via API Gateway) quanto por um entrypoint de
   desenvolvimento local (`cmd/local/main.go`, servidor HTTP comum na
   porta `:8081`) — permite testar o fluxo completo (CPF → JWT → rota
   protegida) sem depender de deploy na AWS a cada mudança.
4. **Correção de autorização**: `GET /orders/customer/:customer_id` passou
   a exigir JWT (`middleware.AuthMiddleware()`). A handler distingue
   `role: "customer"` (só enxerga o próprio `customer_id`, comparado ao
   `user_id` do token) de `role: "admin"` (funcionário, pode consultar
   qualquer cliente).
5. **Revalidação de status a cada chamada, não só na emissão do token**:
   um cliente inativado depois de emitir um token continuava, antes da
   correção, enxergando as próprias OS normalmente — o JWT não expira
   nem é revogado na inativação. `OrderUseCase.GetByCustomerIDChecked`
   revalida o status do cliente em toda chamada do caminho
   `role: "customer"`, devolvendo 403 com a mesma mensagem usada pela
   Lambda ao recusar emissão de token pra cliente inativo. O caminho do
   funcionário (`role: "admin"`) não tem essa restrição — a oficina
   precisa poder consultar o histórico mesmo de um cliente inativado.
6. **Rotas de acompanhamento por e-mail continuam públicas de propósito**
   (`GET /orders/:id/status`, `PATCH /orders/:id/approve`) — herdadas do
   desenho da Fase 2, protegidas só pelo UUID da própria OS. Endurecer
   essas rotas quebraria o propósito do fluxo (link de e-mail sem login),
   e elas não expõem nenhum dado que o UUID já não revele.

## Consequências

- Dois emissores de JWT (`/auth/login` e a Lambda), um único validador —
  `pkg/auth` e `AuthMiddleware` não sabem nem precisam saber qual dos dois
  emitiu o token, só validam assinatura e claims.
- `role: "customer"` fica com acesso deliberadamente mais restrito que
  `role: "admin"` nas mesmas rotas — a autorização é sempre verificada na
  handler, nunca assumida a partir de "tem token válido".
- A Function Serverless depende do mesmo RDS PostgreSQL da API principal
  (consulta direta, sem passar pela API) — acoplamento aceito
  conscientemente porque simplifica o fluxo e evita uma chamada HTTP
  extra só para checar existência/status.
- Testável localmente de ponta a ponta (CPF → JWT → rota protegida,
  incluindo o caso de cliente inativo) sem depender de deploy AWS —
  reduz o custo de iteração durante o desenvolvimento.
