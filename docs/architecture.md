# Arquitetura

Este documento traz os diagramas da arquitetura da Fase 3: componentes da
aplicação com a visão de nuvem completa (APIs, banco, monitoramento),
infraestrutura provisionada no Kubernetes, e o fluxo de deploy da
pipeline. O diagrama de sequência (autenticação por CPF + abertura de OS)
está em [`sequence-diagram.md`](./sequence-diagram.md).

## Sobre o modelo usado (C4)

Os diagramas seguem a ideia do [C4 Model](https://c4model.com/), que separa
a arquitetura em níveis de zoom — aqui usamos dois deles:

- **Componentes** (nível Component/Container do C4): o que existe na
  nuvem, como os 4 repositórios se conectam, e como cada requisito
  obrigatório da Fase 3 (API Gateway, Function Serverless, banco
  gerenciado, monitoramento) se encaixa.
- **Deployment**: onde cada peça roda de fato dentro do cluster —
  namespace, pods, HPA, DaemonSet do New Relic.

O terceiro diagrama (fluxo de deploy) é um fluxo do pipeline de CI/CD.

## Componentes — visão de nuvem, APIs, banco e monitoramento

A API principal segue arquitetura em camadas (Handler → Use Case →
Repository — ver [ADR-002](./adr/ADR-002-padrao-comunicacao.md)). O
webhook de e-mail entra pela mesma porta que qualquer outro adapter: passa
por um middleware próprio de autenticação e cai no mesmo Use Case usado
pelo endpoint HTTP direto — não existe caminho paralelo de regras de
negócio para atualizações vindas por e-mail. A autenticação por CPF é
isolada num repositório e numa Function Serverless próprios, atrás de um
API Gateway dedicado (ver [RFC-003](./rfc/RFC-003-estrategia-autenticacao.md)).

```mermaid
flowchart TB
    Cliente(["Cliente / Oficina<br/>(app, Postman, e-mail)"])

    subgraph AWS["AWS"]
        subgraph AuthFlow["lambda-auth-autoshop"]
            APIGW["API Gateway<br/>(HTTP API)"]
            Lambda["Lambda auth<br/>(valida CPF, consulta<br/>existência/status, emite JWT)"]
        end

        subgraph EKS["infra-k8s-autoshop — cluster EKS"]
            LB["Service (NodePort)"]
            subgraph NS["namespace: autoshop"]
                subgraph Pods["app-autoshop — Deployment autoshop-api<br/>2-6 pods, HPA (cpu 70% / mem 80%)"]
                    API["AutoShop API (Go/Gin)<br/>AuthMiddleware + nrgin + logs JSON"]
                end
            end
            NRI["New Relic Infrastructure<br/>(Helm nri-bundle, DaemonSet)"]
        end

        RDS[("infra-db-autoshop<br/>RDS PostgreSQL")]
        SM["Secrets Manager<br/>(credencial RDS)"]
    end

    NewRelic(["New Relic<br/>APM + dashboards + alertas"])
    Email(["Provedor de e-mail<br/>(webhook status)"])

    Cliente -->|"POST /auth {cpf}"| APIGW --> Lambda
    Lambda -->|"SELECT customer"| RDS
    Lambda -.->|"JWT"| Cliente
    Lambda -->|"lê credencial (IRSA)"| SM

    Cliente -->|"REST + JWT"| LB --> API
    Email -->|"webhook + secret"| API
    API -->|"pgx"| RDS
    API -->|"lê credencial (IRSA)"| SM

    API -->|"traces, métricas,<br/>eventos de negócio"| NewRelic
    NRI -->|"CPU/memória<br/>dos nodes/pods"| NewRelic
```

## Infraestrutura provisionada (Kubernetes)

Formato comum aos ambientes `local`/`ci` (cluster `kind`, efêmero, criado
dentro do próprio runner do GitHub Actions) e `aws` (EKS real,
`infra-k8s-autoshop/infra/aws`) — a mesma estrutura de manifestos
(`app-autoshop/k8s`) é aplicada nos dois, o que muda é o Terraform que
provisiona o cluster por trás.

O `Service` da API é `NodePort` nos dois ambientes (não só no `local`) —
decisão deliberada pra manter o mesmo manifesto entre `local`/`ci` e `aws`
sem duplicar overlay só por causa do tipo de Service. Na AWS, os nodes do
EKS ficam em sub-rede pública com IP público próprio, então o `NodePort`
já é alcançável de fora sem custo adicional de um Load Balancer gerenciado
— a porta é liberada pontualmente no security group dos nodes.

```mermaid
flowchart TB
    GHCR[("GHCR<br/>ghcr.io/.../autoshop-api")]
    Terraform["Terraform<br/>(kind local/CI, ou EKS na AWS)"]

    subgraph Cluster["Cluster Kubernetes"]
        subgraph NS["namespace: autoshop"]
            CM["ConfigMap<br/>autoshop-config"]
            Secret["Secret<br/>autoshop-secrets"]
            SA["ServiceAccount<br/>autoshop-api<br/>(IRSA na AWS)"]
            Deploy["Deployment autoshop-api<br/>2-6 pods"]
            Svc["Service (NodePort)"]
            HPA["HorizontalPodAutoscaler<br/>alvo: cpu 70% / mem 80%"]
            Job["Job db-migrate<br/>aplica as migrations SQL"]
        end
        subgraph NRNS["namespace: newrelic"]
            NRIBundle["Helm nri-bundle<br/>(DaemonSet + kube-state-metrics)"]
        end
        MetricsServer["metrics-server (Helm)"]
    end

    PostgresLocal[("Postgres<br/>docker-compose — local")]
    RDSAWS[("RDS PostgreSQL<br/>— AWS")]

    Terraform -->|"provisiona"| Cluster
    GHCR -->|"pull da imagem"| Deploy
    Svc --> Deploy
    Job --> PostgresLocal
    Job -.->|"na AWS, em vez do<br/>Postgres local"| RDSAWS
    Deploy -->|"lê/grava"| PostgresLocal
    Deploy -.-> RDSAWS
    MetricsServer -->|"métricas de CPU/mem"| HPA
    HPA -->|"escala"| Deploy
    NRIBundle -->|"métricas do cluster"| Deploy
    CM --> Deploy
    Secret --> Deploy
    SA --> Deploy
```

## Fluxo de deploy

A cada push na `main`, a pipeline testa, publica a imagem e faz o deploy
num cluster efêmero criado dentro do próprio runner do GitHub Actions —
sem depender de custo ou credencial de AWS para a homologação. O caminho
`deploy-aws` existe e está habilitado (`AWS_DEPLOY_ENABLED=true` em
`app-autoshop` e `infra-k8s-autoshop`), assumindo a IAM role compartilhada
via OIDC (sem access key estática no GitHub).

```mermaid
flowchart LR
    Push(["git push → main"])

    subgraph CI["GitHub Actions"]
        Test["Job: test<br/>go test + cobertura + SonarCloud"]
        Gate{"Quality Gate<br/>passou?"}
        Build["Job: build-and-push<br/>docker build → GHCR"]
        DeployLocal["Job: deploy-local<br/>cluster kind efêmero no runner"]
        DeployAWS["Job: deploy-aws<br/>assume role via OIDC → kubectl apply no EKS"]
    end

    Health(["Smoke test<br/>curl /health"])
    Destroy(["terraform destroy<br/>(cluster efêmero)"])
    Fail(["Pipeline falha"])

    Push --> Test --> Gate
    Gate -->|"sim"| Build
    Gate -->|"não"| Fail
    Build --> DeployLocal --> Health --> Destroy
    Build -->|"se AWS_DEPLOY_ENABLED=true"| DeployAWS --> Health
```
