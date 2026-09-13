# Arquitetura

Este documento traz os três diagramas da arquitetura proposta: componentes
da aplicação, infraestrutura provisionada e fluxo de deploy.

## Sobre o modelo usado (C4)

Os diagramas seguem a ideia do [C4 Model](https://c4model.com/), que separa
a arquitetura em níveis de zoom — aqui usamos dois deles:

- **Componentes** (nível Component/Container do C4): o que existe *dentro*
  da aplicação e como as peças conversam entre si.
- **Deployment**: onde cada peça roda de fato — cluster, pods, banco,
  registro de imagem.

O terceiro diagrama (fluxo de deploy) — é um fluxo do pipeline de CI/CD.

## Componentes da aplicação

A API segue arquitetura em camadas (Handler → Use Case → Repository). O
webhook de e-mail entra pela mesma porta que qualquer outro adapter: passa
por um middleware próprio de autenticação e cai no mesmo Use Case usado
pelo endpoint HTTP direto — não existe um caminho paralelo de regras de
negócio para atualizações vindas por e-mail.

```mermaid
flowchart LR
    Cliente(["Cliente / Oficina<br/>(REST + Swagger)"])
    EmailProvider(["Provedor de e-mail<br/>(webhook)"])
    DynamoDB[("DynamoDB<br/>5 tabelas")]

    subgraph API["AutoShop API (Go / Gin)"]
        AuthMW["AuthMiddleware (JWT)"]
        WebhookMW["EmailWebhookMiddleware (secret)"]

        subgraph Handlers["Handlers"]
            H1["Auth / Customer / Vehicle /<br/>Service / Part / Order Handler"]
            H2["EmailHandler (webhook)"]
            H3["HealthHandler"]
        end

        subgraph UseCases["Use Cases"]
            UC["Customer / Vehicle / Service /<br/>Part / Order UseCase"]
        end

        subgraph Repos["Repositories"]
            R["Customer / Vehicle / Service /<br/>Part / Order Repository"]
        end
    end

    Cliente --> AuthMW --> H1
    EmailProvider --> WebhookMW --> H2
    H1 --> UC
    H2 -->|"mesmo usecase do endpoint direto"| UC
    UC --> R --> DynamoDB
```

## Infraestrutura provisionada

Formato comum aos ambientes `local` e `ci` (cluster kind). O ambiente
`aws` (fora do escopo desta entrega) segue a mesma estrutura, trocando o
kind por EKS e o DynamoDB Local por tabelas reais — indicado pela linha
pontilhada.

```mermaid
flowchart TB
    GHCR[("GHCR<br/>ghcr.io/.../autoshop-api")]
    Terraform["Terraform<br/>(providers kind + kubectl + helm)"]

    subgraph Cluster["Cluster Kubernetes (kind local/CI, ou EKS na AWS)"]
        subgraph NS["namespace: autoshop"]
            CM["ConfigMap<br/>autoshop-config"]
            Secret["Secret<br/>autoshop-secrets"]
            SA["ServiceAccount<br/>autoshop-api"]
            Deploy["Deployment autoshop-api<br/>2–6 pods"]
            Svc["Service (NodePort/LB)"]
            HPA["HorizontalPodAutoscaler<br/>alvo: cpu 70% / mem 80%"]
            DB["DynamoDB Local<br/>(Deployment + Service)<br/>— local/CI"]
            Job["Job dynamodb-setup<br/>cria as 5 tabelas"]
        end
        MetricsServer["metrics-server (Helm)"]
    end

    DynamoDBAWS[("DynamoDB real<br/>5 tabelas — só AWS")]

    Terraform -->|"kubectl_manifest / helm_release"| NS
    Terraform -->|"provisiona"| Cluster
    GHCR -->|"pull da imagem"| Deploy
    Svc --> Deploy
    Deploy -->|"lê/grava"| DB
    Deploy -.->|"na AWS, em vez do DynamoDB Local"| DynamoDBAWS
    Job --> DB
    MetricsServer -->|"métricas de CPU/mem"| HPA
    HPA -->|"escala"| Deploy
    CM --> Deploy
    Secret --> Deploy
    SA --> Deploy
```

## Fluxo de deploy

A cada push na `master`, a pipeline testa, publica a imagem e faz o
deploy num cluster efêmero criado dentro do próprio runner do GitHub
Actions — sem depender de custo ou credencial de AWS. O caminho AWS
(`deploy-aws`) existe e está pronto, mas fica desligado por padrão.

```mermaid
flowchart LR
    Push(["git push → master"])

    subgraph CI["GitHub Actions"]
        Test["Job: test<br/>go test + cobertura + SonarCloud"]
        Gate{"Quality Gate<br/>passou?"}
        Build["Job: build-and-push<br/>docker build → GHCR"]
        DeployLocal["Job: deploy-local<br/>cluster kind efêmero no runner"]
        DeployAWS["Job: deploy-aws<br/>(desligado por padrão)"]
    end

    Health(["Smoke test<br/>curl /health"])
    Destroy(["terraform destroy<br/>(cluster efêmero)"])
    Fail(["Pipeline falha"])

    Push --> Test --> Gate
    Gate -->|"sim"| Build
    Gate -->|"não"| Fail
    Build --> DeployLocal
    Build -.->|"se AWS_DEPLOY_ENABLED=true"| DeployAWS
    DeployLocal --> Health --> Destroy
```
