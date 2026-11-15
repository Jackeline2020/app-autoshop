#!/bin/bash
# Mostra os comandos que o orquestrador publicou numa fila (e os remove).
# Uso: read-commands.sh execution-commands | billing-commands
set -e
url=$(awslocal sqs get-queue-url --queue-name "${1:?informe a fila}" --query QueueUrl --output text)
while true; do
  out=$(awslocal sqs receive-message --queue-url "$url" --max-number-of-messages 10 --wait-time-seconds 1)
  [ -z "$out" ] && break
  echo "$out" | python3 -c 'import json,sys
for m in json.load(sys.stdin).get("Messages", []):
    b = json.loads(m["Body"]); print(b["type"], "->", json.dumps(b.get("payload"), ensure_ascii=False))'
  for rh in $(echo "$out" | python3 -c 'import json,sys
for m in json.load(sys.stdin).get("Messages", []): print(m["ReceiptHandle"])'); do
    awslocal sqs delete-message --queue-url "$url" --receipt-handle "$rh"
  done
done
