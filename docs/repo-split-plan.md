# Plano de split em 4 repositórios

A Fase 3 exige 4 repositórios separados, cada um com CI/CD próprio e deploy automático:

1. `lambda-auth` — Function Serverless
2. `infra-k8s` — Infraestrutura Kubernetes (Terraform)
3. `infra-db` — Infraestrutura do Banco de Dados Gerenciado (Terraform)
4. `app` — Aplicação principal executando em Kubernetes

Este documento descreve **o que vai para cada repositório quando o split for feito de verdade** (ainda não foi — ver nota no fim). Serve de referência pra não perder nenhuma peça na hora de migrar, e documenta as decisões de dependência entre repos, que é justamente o tipo de coisa que vale a pena explicar no vídeo.

## 1. `app` — aplicação principal

O que sai do monorepo pra cá:

- `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum`
- `migrations/` — o schema SQL é acoplado ao código Go que o consome (repository layer), não à infra do banco
- `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env.example`
- `docs/` inteiro — `architecture.md`, `database/er-diagram.md`, `rfc/`, `adr/`, DDD (Domain Storytelling, Event Storming, Ubiquitous Language), `SECURITY_REPORT.md`
- `sonar-project.properties`

**Por que a documentação toda fica aqui, mesmo tendo pedaços sobre infra**: fragmentar RFC/ADR entre 4 repos torna difícil montar a "documentação arquitetural completa" pedida no enunciado. Cada um dos outros 3 repositórios linka de volta pro `docs/` deste repo no próprio README, em vez de duplicar conteúdo.

**CI/CD deste repo**: build, testes (unitário + integração contra um Postgres descartável, igual já está hoje em `ci-cd.yml`), SonarCloud, build+push da imagem pro GHCR. Depois do push da imagem, esse pipeline **dispara** o do `infra-k8s` (ver seção de dependências entre repos abaixo) — é este repo que decide "existe uma imagem nova pronta pra ir pro ar".

## 2. `infra-k8s` — cluster e manifestos da aplicação

- `infra/local/`, `infra/ci/`, e a parte de `infra/aws/` referente só ao cluster: `eks.tf`, `network.tf`, `github-oidc.tf`, `provider.tf`, `variables.tf`/`outputs.tf` (os campos de cluster, não os de banco)
- `k8s/` inteiro (`base/` + `overlays/local` e `overlays/aws`) — os manifestos do Deployment/Service/HPA/ConfigMap/Secret da API viajam junto com quem os aplica (`kubectl_manifest` no Terraform já lê esses arquivos via `file()`)

**CI/CD deste repo**: `terraform validate`/`plan` em PR; `apply` (cluster + manifestos, exceto a migration — ver decisão abaixo) em push na branch principal, ou disparado pelo `app` quando uma imagem nova sai.

## 3. `infra-db` — banco gerenciado

- De `infra/aws/`: só `rds.tf` + as variáveis/outputs específicos de banco (`db_name`, `db_username`, `db_instance_class`, `rds_endpoint`, `rds_secret_arn`)
- Precisa de um `provider.tf` próprio (hoje compartilhado com `eks.tf` no mesmo projeto Terraform)

**Dependência que muda com o split**: hoje `rds.tf` referencia `module.eks.node_security_group_id` diretamente, porque cluster e banco vivem no mesmo projeto Terraform. Com `infra-k8s` em outro repositório, isso vira uma variável de entrada (`eks_node_security_group_id`) que `infra-k8s` expõe como output e `infra-db` recebe — via Terraform remote state (`terraform_remote_state` apontando pro state do `infra-k8s`) ou, mais simples de operar num projeto de estudo, colada manualmente como `TF_VAR_eks_node_security_group_id` depois do primeiro apply do `infra-k8s`, do mesmo jeito que `IRSA_ROLE_ARN` já é colado manualmente hoje.

**CI/CD deste repo**: `terraform validate`/`plan` em PR, `apply` em push na branch principal.

## 4. `lambda-auth` — function serverless de autenticação

Repositório novo (ainda não existe nenhum código dele no monorepo — é o que a tarefa 5 vai criar):

- Código Go da Lambda (valida CPF, consulta cliente no Postgres, gera JWT) — reaproveitando a lógica de `pkg/validator/document.go` e `pkg/auth/jwt.go`, mas como cópia própria neste repo (repos separados não compartilham módulo Go facilmente sem publicar um pacote — duplicar esse pedaço pequeno é mais simples que criar um 5º repositório só pra lib compartilhada)
- Terraform da própria Lambda + API Gateway (`aws_lambda_function`, `aws_apigatewayv2_api`, role/policy IAM da function) — autocontido, não depende dos outros repos além de saber o `rds_secret_arn` do `infra-db` (mesmo padrão de variável colada manualmente)

**CI/CD deste repo**: build + teste do Go, `terraform apply` da Lambda + Gateway.

## Decisão em aberto: quem roda a migration do banco

Hoje o Job `db-migrate` (dentro do `infra/local`/`infra/ci`) lê `migrations/000001_init_schema.up.sql` via `file()` — funciona porque tudo está no mesmo checkout. Depois do split, `infra-k8s` não tem mais acesso a esse arquivo (ele mora no repo `app`).

Três formas de resolver, da mais simples à mais "correta":

1. **(Recomendado pra este projeto) A migration sai do Terraform do `infra-k8s` e vira um passo do pipeline do `app`.** Depois que `infra-k8s` provisiona o cluster (ou confirma que já existe), o workflow do `app` faz `kubectl apply` do Job de migration diretamente (usando um kubeconfig que `infra-k8s` expõe como secret do repositório). Mantém a regra "quem dona o schema, aplica o schema" — o `app` já é dono de `migrations/`.
2. `infra-k8s` faz `actions/checkout` de um segundo repositório (`app`) só pra pegar a pasta `migrations/` no momento do apply. Funciona, mas cria acoplamento de checkout cruzado logo na primeira pipeline.
3. Publicar `migrations/*.sql` como artifact/release do repo `app` e o `infra-k8s` baixar esse artifact antes do apply. Mais "correto" architecturally, mais peça se movendo pra um projeto de estudo.

Fica registrado aqui como decisão: usar a opção 1 quando o split acontecer de verdade.

## Regras de proteção de branch (a configurar manualmente em cada repositório real)

Pra cada um dos 4 repositórios, nas configurações do GitHub:

- Branch `main`/`master` protegida — sem commit direto
- Pull Request obrigatório pra merge
- (Opcional, mas recomendado) exigir que o CI passe antes do merge

Isso não dá pra automatizar por Terraform/código a partir daqui — é configuração de repositório que só existe depois que os 4 repositórios forem criados de verdade no GitHub.

## Nota importante

Este é só o **plano** — os arquivos continuam todos juntos no repositório `autoshop` por enquanto. A reorganização física (criar os 4 repositórios de verdade e mover os arquivos pra cada um) fica pra quando: (a) a ponte com o seu computador estiver com a shell de volta (pra eu poder mover pastas), e (b) você tiver criado os 4 repositórios no GitHub — criar repositório novo é algo que eu não tenho permissão de fazer por aqui.
