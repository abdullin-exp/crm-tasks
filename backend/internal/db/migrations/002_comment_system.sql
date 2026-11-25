-- Системные комментарии (перевод задачи и т.п.)
ALTER TABLE tasks.task_comment ADD COLUMN is_system boolean NOT NULL DEFAULT false;
