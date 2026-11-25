#!/usr/bin/env sh
# Отправляет событие создания задачи в Kafka CRM (контейнер broker).
# Использование:
#   ./scripts/send-kafka-event.sh                      # демо-событие «Заявка с почты»
#   ./scripts/send-kafka-event.sh '{"type_code":"missed_call","data":{"phone":"+79270000000"}}'
set -eu

TOPIC="${KAFKA_TOPIC:-crm-tasks.task.create}"
EVENT_ID="evt-$(date +%s)"
DEFAULT=$(cat <<EOF
{"event_id":"$EVENT_ID","type_code":"email_order","title":"Клиент разместил заказ через почту","description":"Задача создана по автоматически классифицированному письму","priority":2,"data":{"order_number":"2611-345228-76027","order_sum":5099,"email":"client@example.com","client_kind":"b2b","products":[{"name":"Диск пильный 260x3.2x30","code":"28270274","qty":1,"accuracy":100}]}}
EOF
)
MSG="${1:-$DEFAULT}"

echo "$MSG" | docker exec -i broker /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic "$TOPIC"
echo "Отправлено в $TOPIC: $MSG"
