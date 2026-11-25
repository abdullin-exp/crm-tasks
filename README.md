# crm-tasks — сервис задач с конструктором форм (MVP)

Новый сервис задач для CRM. Главное отличие от задач CRM: **тип задачи и его форма собираются в конструкторе**
(блоки → поля разных типов), без написания кода и деплоя. Анализ текущих задач и принятые решения — [docs/analysis.md](docs/analysis.md).

- **Backend**: Go 1.23 (chi, pgx, kafka-go)
- **Frontend**: Vue 3 + Vite + Element Plus
- **БД**: Postgres CRM (схема `tasks`, таблицы CRM только читаются)
- **Kafka**: брокер CRM (`broker:9092`)
- Авторизации нет.

## Запуск

Нужны контейнеры CRM (сеть `crm_crm`):

```sh
make crm-up      # postgres + kafka из ../crm
make up          # сборка и запуск crm-tasks
```

| Что | Адрес |
|---|---|
| UI | http://localhost:8091 |
| API напрямую | http://localhost:8090/api |
| Kafka UI (из CRM) | http://localhost:8088 (`cd ../crm && make up-kafka`) |

Порты меняются переменными `FRONTEND_PORT`, `BACKEND_PORT`. Без Kafka: `KAFKA_ENABLED=false make up`.

При первом запуске применяются миграции и создаются два демо-типа: «Заявка с почты» (повторяет карточку из CRM)
и «Перезвон на пропущенный звонок».

## Сценарий проверки

1. **Конструктор типов** → «Новый тип» → название/код → откроется конструктор.
2. Перетащите (или кликните) блоки и поля из палитры, настройте справа: название, код, ширину, обязательность,
   варианты, колонки таблицы, «колонка в списке». Вкладка «Предпросмотр» — форма глазами менеджера.
3. «Настройки и результаты» — срок решения, приоритет, кнопки результатов (закрывающие / оставляющие в работе).
4. «Сохранить» → «Создать задачу этого типа» → заполнить → «Создать задачу».
5. В карточке: изменить поля, выбрать результат, добавить комментарий — изменения видны в «Истории изменений».
6. Kafka: `make kafka-event` (или пример со вкладки «Kafka / API» конструктора) — задача появится в списке с источником «Kafka».

## Модель

```
tasks.task_type          тип: код, название, срок, приоритет, form_schema (jsonb), form_version
tasks.task_type_result   результаты типа: название, цвет, is_closing, порядок
tasks.task               задача: системные колонки (как в doc_task) + data jsonb (значения полей конструктора)
tasks.task_comment       комментарии
tasks.task_event         журнал изменений (дифф полей)
tasks.task_status        Новая / В работе / Выполнена (как enum_task_status)
```

Схема формы:

```json
{"blocks": [
  {"id": "main", "title": "Основная информация", "kind": "fields", "span": 24, "collapsible": false,
   "fields": [
     {"id": "f1", "type": "system", "system": "title", "label": "Цель задачи", "required": true, "span": 12},
     {"id": "f2", "code": "order_sum", "type": "money", "label": "Сумма заказа", "span": 12, "show_in_list": true}
   ]},
  {"id": "comments", "title": "Комментарии", "kind": "comments", "span": 12}
]}
```

- **Виды блоков**: `fields`, `comments`, `history`, `contractor_tasks` (задачи нового сервиса + старые `doc_task`), `client_activity` (звонки, заказы, задачи из CRM).
- **Типы полей**: строка, текст, число, сумма, дата, дата-время, флажок, список, мультивыбор, переключатель, телефон, e-mail, ссылка, сотрудник (`ref_user`), контрагент (`ref_contractor`), таблица (табличная часть), информационный текст.
- **Системные поля** (колонки задачи): цель, описание, результат, исполнитель, контрагент, контактное лицо, плановая дата, приоритет, наблюдатели (`observer_user_ids`, таблица `tasks.task_observer`); только чтение — статус, номер, создано, факт. завершение.

**Условная видимость**: у поля может быть `"visible_if": {"field": "result_id", "op": "in", "values": ["4","5"]}`.
Источник — пользовательское поле или системное (результат, приоритет, исполнитель, контрагент, КЛ, срок, цель, описание);
операторы `eq, neq, in, not_in, filled, empty`; цепочки учитываются (скрыт источник → скрыто зависимое).
Скрытое поле не проверяется на обязательность, его значение сохраняется. В демо «Причина отказа» показывается и обязательна
только при результате-отказе.

Значения валидируются на бэкенде по схеме (тип, обязательность, варианты, min/max, формат телефона/e-mail/даты),
одинаково для UI, REST и Kafka. При изменении формы данные удалённых полей у существующих задач сохраняются.

## Создание задач

**Вручную** — UI («Создать задачу» в списке).

**Kafka** — топик `crm-tasks.task.create`:

```json
{
  "event_id": "evt-123",
  "type_code": "email_order",
  "title": "Клиент разместил заказ через почту",
  "executor_user_id": "uuid ref_user",
  "contractor_id": "uuid ref_contractor",
  "contact_contractor_id": "uuid",
  "solve_plan_at": "2026-11-25T18:00:00+03:00",
  "priority": 2,
  "observer_user_ids": ["uuid ref_user"],
  "data": {"order_number": "2611-345228-76027", "order_sum": 5099}
}
```

- Повтор с тем же `event_id` не создаёт дубль (без `event_id` ключом служит `topic:partition:offset`).
- Невалидные сообщения → `crm-tasks.task.create.dlq` с заголовком `error` (`make kafka-dlq`).
- Временные ошибки (БД недоступна) — повтор с backoff без коммита offset.
- Пустой `title` → название типа; `solve_plan_at` не задан → сейчас + срок типа.

**REST** — `POST /api/tasks` с тем же телом (`external_id` вместо `event_id` → идемпотентно).

## Исходящие события (Kafka)

Изменения задач публикуются в топик `crm-tasks.task.events` (`KAFKA_EVENTS_TOPIC`) — CRM и другие сервисы могут реагировать,
например, на закрытие задачи.

| `event_type` | Когда |
|---|---|
| `task.created` | создана (вручную, Kafka, API) |
| `task.updated` | изменены поля |
| `task.closed` | выбран закрывающий результат |
| `task.reopened` | выполненная задача возвращена в работу |
| `task.taken` | взята в работу |
| `task.transferred` | переведена на другого исполнителя |

```json
{"event_id": "uuid", "event_type": "task.closed", "occurred_at": "2026-11-25T14:20:00+03:00",
 "task": { /* полный снимок задачи, как в GET /api/tasks/{id} */ },
 "details": {"changes": [{"field": "result_id", "label": "Результат", "old": null, "new": "Заказ оформлен"}]}}
```

- Ключ сообщения — id задачи: события одной задачи идут в одну партицию и не перемешиваются. Заголовки `event_type`, `event_id`.
- **Transactional outbox** (`tasks.outbox`): событие пишется в той же транзакции, что и изменение, и отправляется фоновым
  публикатором. Если Kafka недоступна, изменение задачи всё равно проходит, а событие уйдёт после восстановления.
  Доставка at-least-once — потребителю нужно дедуплицировать по `event_id`. Отправленные записи чистятся через 7 дней.
- Сколько событий отправлено и сколько ждёт отправки — в `GET /api/health` (`kafka.publisher`).

## API

| Метод | Путь | |
|---|---|---|
| GET | `/api/meta` | справочники конструктора |
| GET/POST | `/api/task-types` | типы |
| GET/PUT/DELETE | `/api/task-types/{id}` | тип + результаты |
| PUT | `/api/task-types/{id}/form` | сохранить схему формы |
| GET | `/api/tasks/stats` | счётчики по статусам и просрочке (те же фильтры, что у списка) |
| GET/POST | `/api/tasks` | список (фильтры: `type_id, status_id, result_id, executor_user_id, observer_user_id, contractor_id, source, q, overdue, created_from/to, sort, order, page, per_page`) / создание |
| GET/PUT | `/api/tasks/{id}` | задача + тип / обновление |
| POST | `/api/tasks/{id}/status` | «Новая» / «В работе» |
| POST | `/api/tasks/{id}/transfer` | перевод на другого исполнителя: `{executor_user_id, reason, solve_plan_at?}` |
| POST | `/api/tasks/bulk-transfer` | массовый перевод: `{task_ids[], executor_user_id, reason, solve_plan_at?}` → `{transferred, errors{id: причина}}` |
| GET/POST | `/api/tasks/{id}/comments` | комментарии (`PATCH/DELETE /api/comments/{id}`) |
| GET | `/api/tasks/{id}/events` · `/contractor-tasks` · `/client-activity` | данные блоков |
| GET | `/api/refs/users` · `/api/refs/contractors?kind=juristic\|physical` | справочники CRM |
| GET | `/api/health` | БД + состояние консьюмера Kafka |

## Разработка

```sh
make test                                   # unit-тесты (валидация схемы и данных)
cd frontend && npm i && npm run dev         # Vite на :5173, /api проксируется на :8090
```
