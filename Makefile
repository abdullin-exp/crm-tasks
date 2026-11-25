.DEFAULT_GOAL := help
CRM_DIR ?= ../crm

## help: список команд
help:
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## crm-up: поднять БД и Kafka из CRM (сеть crm_crm)
crm-up:
	cd $(CRM_DIR) && docker compose up -d postgres && docker compose --profile kafka up -d kafka

## up: собрать и запустить сервис задач
up:
	docker compose up -d --build

## down: остановить сервис задач
down:
	docker compose down

## logs: логи бэкенда
logs:
	docker compose logs -f backend

## test: unit-тесты бэкенда
test:
	docker run --rm -v "$(PWD)/backend":/src -v crm-tasks-gomod:/go/pkg/mod -w /src golang:1.23-alpine go test ./...

## kafka-event: отправить демо-событие создания задачи
kafka-event:
	./scripts/send-kafka-event.sh

## kafka-dlq: показать отклонённые сообщения
kafka-dlq:
	docker exec broker /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 \
		--topic crm-tasks.task.create.dlq --from-beginning --property print.headers=true --timeout-ms 5000 || true

## db-reset: удалить схему tasks (все данные сервиса!) — пересоздастся при старте
db-reset:
	@echo -n 'Удалить схему tasks со всеми данными? [y/N] ' && read ans && [ $${ans:-N} = y ]
	docker exec crm-postgres-1 psql -U crm -d crm -c 'DROP SCHEMA tasks CASCADE'
	docker compose restart backend

.PHONY: help crm-up up down logs test kafka-event kafka-dlq db-reset
