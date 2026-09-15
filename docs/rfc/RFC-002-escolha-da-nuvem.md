# RFC-002 — Escolha da nuvem (AWS)

| | |
|---|---|
| **Status** | Aceito |
| **Fase** | 3 |
| **Autores** | Jackeline Carvalho |
| **Relacionado a** | [RFC-001 — PostgreSQL](./RFC-001-postgres-database.md), [RFC-003 — Estratégia de autenticação](./RFC-003-estrategia-autenticacao.md), [ADR-003 — Uso de HPA](../adr/ADR-003-uso-hpa.md) |

## Contexto

A Fase 3 exige, ao mesmo tempo: um API Gateway, uma Function Serverless, um
banco de dados gerenciado, um cluster Kubernetes com escalabilidade, e
Terraform pra provisionar tudo isso — com "livre escolha de nuvem". Nas
Fases 1/2 o projeto já rodava com foco em AWS (IAM, considerações de
DynamoDB real), então a decisão aqui é menos "qual nuvem" e mais
"confirmar e justificar por que continuar na AWS faz sentido pra esse
conjunto específico de requisitos".

## Problema

Cada requisito obrigatório da Fase 3 tem um equivalente gerenciado em mais
de um provedor (Lambda vs. Cloud Functions vs. Azure Functions; API
Gateway vs. Cloud Endpoints; RDS vs. Cloud SQL; EKS vs. GKE/AKS). Trocar de
provedor no meio da Fase 3 significaria reaprender IAM, rede e Terraform
providers do zero, sem ganho técnico correspondente — o risco maior nesse
ponto do projeto é o tempo, não a escolha em si.

## Decisão

Manter a **AWS** como provedor único para todos os 4 repositórios,
usando:

1. **AWS Lambda** (`lambda-auth-autoshop`) — Function Serverless de
   autenticação por CPF, atrás de um **API Gateway HTTP API**
   (`aws_apigatewayv2_api`), mais barato e mais simples que uma REST API
   completa pra um único endpoint de auth.
2. **Amazon RDS for PostgreSQL** (`infra-db-autoshop`) — banco gerenciado
   (justificativa completa em [RFC-001](./RFC-001-postgres-database.md)).
3. **Amazon EKS** (`infra-k8s-autoshop`) — cluster Kubernetes gerenciado,
   com node group elástico (`node_min_size`/`node_max_size`) e HPA a nível
   de pod (ver [ADR-003](../adr/ADR-003-uso-hpa.md)).
4. **IAM Roles Anywhere via OIDC** — tanto o GitHub Actions (deploy) quanto
   os pods da API (`autoshop-api-irsa`) assumem roles IAM reais via OIDC,
   sem nenhuma access key estática guardada em secret do GitHub ou do
   cluster.
5. **Secrets Manager** — credencial do RDS, lida pela API via IRSA em vez
   de variável de ambiente em texto puro.

Motivos, além da continuidade com as Fases 1/2:

- **Um provedor único simplifica rede e IAM**: EKS, RDS e Lambda na mesma
  VPC/conta reduzem a superfície de configuração (um security group do
  EKS libera o RDS, uma única cadeia de confiança OIDC serve GitHub
  Actions e IRSA).
- **Terraform providers maduros**: os módulos `terraform-aws-modules/eks`
  e `terraform-aws-modules/iam` (usados em `infra-k8s-autoshop`) cobrem
  boa parte da complexidade de IAM/IRSA que teria que ser escrita à mão em
  outro provedor.
- **Tier gratuito e custo previsível**: RDS `db.t3.micro`/EKS com 2 nodes
  `t3.medium` cabem em orçamento de estudante por tempo limitado — desde
  que a infraestrutura seja destruída após a entrega (ver seção de
  destroy no documento de comandos operacionais).

## Consequências

- Os 4 repositórios usam o provider `hashicorp/aws` (`~> 5.0`) e
  compartilham a mesma conta/região (`us-east-1`).
- Toda a infraestrutura é descartável e recriável via `terraform apply`
  a partir do zero — nenhum recurso foi criado manualmente pelo console
  (com exceção da chave de acesso IAM usada só para autenticar o próprio
  Terraform, que não é um recurso da aplicação).
- Contrapartida aceita conscientemente: ausência de estratégia
  multi-cloud/disaster recovery entre provedores — fora do escopo da
  Fase 3, e não pedido pelo enunciado.
- Caminho de evolução, se o projeto continuasse além da disciplina:
  migrar o backend remoto do Terraform (`terraform.tfstate` local, hoje)
  para um bucket S3 com DynamoDB lock, e separar contas AWS por ambiente
  (dev/hml/prod) em vez de uma conta única.
