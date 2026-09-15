# Divisão em 4 repositórios

A Fase 3 exige 4 repositórios separados, cada um com CI/CD próprio e deploy automático. O monorepo das Fases 1 e 2 foi dividido em:

1. `lambda-auth-autoshop` — Function Serverless de autenticação por CPF
2. `infra-k8s-autoshop` — Infraestrutura Kubernetes (Terraform: cluster EKS + manifestos)
3. `infra-db-autoshop` — Infraestrutura do Banco de Dados Gerenciado (Terraform: RDS)
4. `app-autoshop` — Aplicação principal executando em Kubernetes

Este documento registra as decisões de divisão de responsabilidades entre repositórios e as dependências entre eles.

## 1. `app-autoshop` — aplicação principal

- `cmd/`, `internal/`, `pkg/`, `go.mod`, `go.sum`
- `migrations/` — schema SQL, acoplado ao código Go que o consome (repository layer)
- `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env.example`
- `docs/` — `architecture.md`, `database/er-diagram.md`, `rfc/`, `adr/`, cobrindo a documentação arquitetural completa do sistema
- `sonar-project.properties`

Os outros 3 repositórios linkam de volta pra `docs/` deste repositório no próprio README, em vez de duplicar conteúdo.

**CI/CD**: build, testes (unitário + integração contra um Postgres descartável), SonarCloud, build+push da imagem pro GHCR, e o job `deploy-aws` que aplica os manifestos no cluster EKS já provisionado pelo `infra-k8s-autoshop`.

## 2. `infra-k8s-autoshop` — cluster e manifestos da aplicação

- `infra/local/` (cluster `kind` pra desenvolvimento) e `infra/aws/` (cluster EKS: `eks.tf`, `network.tf`, `github-oidc.tf`, `provider.tf`, `variables.tf`/`outputs.tf`)
- `k8s/` inteiro (`base/` + `overlays/local` e `overlays/aws`) — manifestos de Deployment, Service, HPA, ConfigMap e Secret da API

**CI/CD**: `terraform validate`/`plan` em PR; `apply` (cluster) em push na branch principal.

## 3. `infra-db-autoshop` — banco gerenciado

- `rds.tf` e as variáveis/outputs de banco (`db_name`, `db_username`, `db_instance_class`, `rds_endpoint`)
- `provider.tf` próprio, independente do projeto Terraform do cluster

O acesso à porta 5432 do RDS é liberado só a partir do security group dos nodes do EKS. Esse security group é um output do `infra-k8s-autoshop`, repassado ao `infra-db-autoshop` como variável (mesmo padrão usado para o `IRSA_ROLE_ARN`, passado como secret ao `app-autoshop`).

**CI/CD**: `terraform validate`/`plan` em PR, `apply` em push na branch principal.

## 4. `lambda-auth-autoshop` — function serverless de autenticação

- Código Go da Lambda (valida CPF, consulta cliente no Postgres, gera JWT), reaproveitando a lógica de validação de CPF e geração de JWT do `app-autoshop`
- Terraform da própria Lambda + API Gateway (`aws_lambda_function`, `aws_apigatewayv2_api`, role/policy IAM da function), dependendo do `infra-db-autoshop` só para o endpoint/credencial do RDS

**CI/CD**: build + teste do Go, `terraform apply` da Lambda + Gateway.

## Migration do banco

O Job `db-migrate` lê `migrations/000001_init_schema.up.sql`, que pertence ao `app-autoshop`. Por isso a migration roda como um passo do pipeline do `app-autoshop` (`kubectl apply` do Job, autenticado via IRSA), depois que o `infra-k8s-autoshop` provisiona o cluster — o repositório dono do schema é quem o aplica.

## Regras de proteção de branch

Configuradas nas configurações do GitHub de cada um dos 4 repositórios:

- Branch `main` protegida — sem commit direto
- Pull Request obrigatório pra merge
