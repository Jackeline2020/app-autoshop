#!/bin/bash
# Simula um evento do Execution ou do Billing na fila os-events.
# Uso: send-event.sh <tipo> <order_id> ['<payload json>']
# Ex.: send-event.sh DiagnosisFinished 1a2b... '{"diagnosis":"Pastilha gasta"}'
set -e
type=$1; order=$2
# Junta o restante: o PowerShell quebra o JSON em vários argumentos nos espaços
payload="${*:3}"; payload=${payload:-null}
if [ -z "$type" ] || [ -z "$order" ]; then
  echo "uso: send-event.sh <tipo> <order_id> ['<payload json>']"; exit 1
fi
# Remove barras de aspas escapadas, caso o terminal as repasse
payload=${payload//\\\"/\"}
if ! echo "$payload" | python3 -c 'import json,sys; json.load(sys.stdin)' 2>/dev/null; then
  echo "payload não é um JSON válido: $payload"; exit 1
fi
id=$(cat /proc/sys/kernel/random/uuid)
body="{\"message_id\":\"$id\",\"type\":\"$type\",\"order_id\":\"$order\",\"correlation_id\":\"teste-manual\",\"occurred_at\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"payload\":$payload}"
url=$(awslocal sqs get-queue-url --queue-name os-events --query QueueUrl --output text)
awslocal sqs send-message --queue-url "$url" --message-body "$body" >/dev/null
echo "evento $type enviado para a OS $order (payload: $payload)"
