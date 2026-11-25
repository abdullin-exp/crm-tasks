-- Наблюдатели задачи (аналог cros_task_observer_user в CRM)
CREATE TABLE tasks.task_observer (
    task_id    uuid        NOT NULL REFERENCES tasks.task (id) ON DELETE CASCADE,
    user_id    uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, user_id)
);
CREATE INDEX ix_task_observer_user ON tasks.task_observer (user_id);
