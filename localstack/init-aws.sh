#!/bin/bash
# Cria no LocalStack as filas usadas pelo OS Service (orquestrador da Saga).
# Em produção, as mesmas filas são criadas por Terraform.
set -e

create_queue_with_dlq() {
  local queue=$1
  local dlq_url dlq_arn
  dlq_url=$(awslocal sqs create-queue --queue-name "$queue-dlq" --query QueueUrl --output text)
  dlq_arn=$(awslocal sqs get-queue-attributes --queue-url "$dlq_url" --attribute-names QueueArn --query Attributes.QueueArn --output text)
  awslocal sqs create-queue --queue-name "$queue" --attributes "{
    \"VisibilityTimeout\": \"30\",
    \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"$dlq_arn\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"
  }" >/dev/null
  echo "fila $queue criada (DLQ: $queue-dlq)"
}

# Entrada do orquestrador
create_queue_with_dlq os-events
# Saídas (comandos). Criadas aqui para permitir rodar o serviço isolado; no
# ambiente completo pertencem ao Execution e ao Billing Service.
create_queue_with_dlq execution-commands
create_queue_with_dlq billing-commands
