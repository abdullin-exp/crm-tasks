-- Схема сервиса задач. Таблицы CRM (public) не изменяются, только читаются.
CREATE SCHEMA IF NOT EXISTS tasks;

-- Статусы — те же, что в CRM (enum_task_status), чтобы переход был прозрачным
CREATE TABLE tasks.task_status (
    id       smallint PRIMARY KEY,
    code     varchar(30)  NOT NULL UNIQUE,
    name     varchar(100) NOT NULL,
    is_final boolean      NOT NULL DEFAULT false
);
INSERT INTO tasks.task_status (id, code, name, is_final) VALUES
    (1, 'new', 'Новая', false),
    (2, 'in_progress', 'В работе', false),
    (3, 'done', 'Выполнена', true);

-- Тип задачи + схема формы (конструктор)
CREATE TABLE tasks.task_type (
    id               serial PRIMARY KEY,
    code             varchar(100) NOT NULL UNIQUE,
    name             varchar(255) NOT NULL,
    description      text         NOT NULL DEFAULT '',
    color            varchar(20)  NOT NULL DEFAULT '#409eff',
    is_active        boolean      NOT NULL DEFAULT true,
    default_priority smallint     NOT NULL DEFAULT 2,
    solve_minutes    integer,
    legacy_entity_id integer, -- id типа задачи в CRM (enum_entity), для миграции
    form_schema      jsonb        NOT NULL DEFAULT '{"blocks":[]}',
    form_version     integer      NOT NULL DEFAULT 1,
    created_at       timestamptz  NOT NULL DEFAULT now(),
    updated_at       timestamptz  NOT NULL DEFAULT now()
);

-- Результаты задачи настраиваются на тип (в CRM они зашиты в код каждого класса)
CREATE TABLE tasks.task_type_result (
    id           serial PRIMARY KEY,
    task_type_id integer      NOT NULL REFERENCES tasks.task_type (id) ON DELETE CASCADE,
    name         varchar(255) NOT NULL,
    color        varchar(20)  NOT NULL DEFAULT 'primary',
    is_closing   boolean      NOT NULL DEFAULT true,
    sort         integer      NOT NULL DEFAULT 0,
    is_active    boolean      NOT NULL DEFAULT true
);
CREATE INDEX ix_task_type_result_type ON tasks.task_type_result (task_type_id);

CREATE TABLE tasks.task (
    id                    uuid PRIMARY KEY,
    number                bigserial    NOT NULL UNIQUE,
    task_type_id          integer      NOT NULL REFERENCES tasks.task_type (id),
    form_version          integer      NOT NULL,
    status_id             smallint     NOT NULL DEFAULT 1 REFERENCES tasks.task_status (id),
    result_id             integer REFERENCES tasks.task_type_result (id),
    priority              smallint     NOT NULL DEFAULT 2,
    title                 varchar(500) NOT NULL DEFAULT '',
    description           text         NOT NULL DEFAULT '',
    executor_user_id      uuid,
    contractor_id         uuid,
    contact_contractor_id uuid,
    solve_plan_at         timestamptz,
    solve_fact_at         timestamptz,
    data                  jsonb        NOT NULL DEFAULT '{}',
    source                varchar(20)  NOT NULL DEFAULT 'manual',
    external_id           varchar(255),
    legacy_task_id        uuid,
    created_by            uuid,
    created_at            timestamptz  NOT NULL DEFAULT now(),
    updated_at            timestamptz  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ux_task_source_external ON tasks.task (source, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX ix_task_type ON tasks.task (task_type_id);
CREATE INDEX ix_task_status ON tasks.task (status_id);
CREATE INDEX ix_task_executor_status ON tasks.task (executor_user_id, status_id);
CREATE INDEX ix_task_contractor ON tasks.task (contractor_id);
CREATE INDEX ix_task_contact ON tasks.task (contact_contractor_id);
CREATE INDEX ix_task_solve_plan ON tasks.task (solve_plan_at);
CREATE INDEX ix_task_created ON tasks.task (created_at DESC);
CREATE INDEX ix_task_data ON tasks.task USING gin (data jsonb_path_ops);

CREATE TABLE tasks.task_comment (
    id             uuid PRIMARY KEY,
    task_id        uuid        NOT NULL REFERENCES tasks.task (id) ON DELETE CASCADE,
    text           text        NOT NULL,
    author_user_id uuid,
    is_favorite    boolean     NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_task_comment_task ON tasks.task_comment (task_id, created_at DESC);

-- Журнал изменений задачи
CREATE TABLE tasks.task_event (
    id         bigserial PRIMARY KEY,
    task_id    uuid        NOT NULL REFERENCES tasks.task (id) ON DELETE CASCADE,
    kind       varchar(50) NOT NULL,
    message    text        NOT NULL DEFAULT '',
    payload    jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_task_event_task ON tasks.task_event (task_id, id DESC);
