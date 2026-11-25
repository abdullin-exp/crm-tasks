-- Transactional outbox: события задач пишутся в той же транзакции, что и изменение,
-- фоновый публикатор отправляет их в Kafka (crm-tasks.task.events).
CREATE TABLE tasks.outbox (
    id         bigserial PRIMARY KEY,
    event_id   uuid        NOT NULL UNIQUE,
    event_type varchar(50) NOT NULL,
    task_id    uuid        NOT NULL,
    payload    jsonb       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at    timestamptz,
    attempts   integer     NOT NULL DEFAULT 0,
    last_error text
);
CREATE INDEX ix_outbox_unsent ON tasks.outbox (id) WHERE sent_at IS NULL;
