package config

import (
	"os"
	"strings"
)

// Config настройки сервиса, читаются из переменных окружения.
type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	CRMSchema        string // схема CRM, из которой читаются справочники (пользователи, контрагенты, старые задачи)
	CRMURL           string // базовый URL CRM для ссылок на старые задачи
	KafkaEnabled     bool
	KafkaBrokers     []string
	KafkaTopic       string
	KafkaDLQTopic    string
	KafkaGroupID     string
	KafkaEventsTopic string
	SeedDemo         bool
}

func Load() Config {
	return Config{
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://crm:crm@postgres:5432/crm?sslmode=disable"),
		CRMSchema:        env("CRM_SCHEMA", "public"),
		CRMURL:           strings.TrimRight(env("CRM_URL", "http://localhost"), "/"),
		KafkaEnabled:     env("KAFKA_ENABLED", "true") == "true",
		KafkaBrokers:     strings.Split(env("KAFKA_BROKERS", "broker:9092"), ","),
		KafkaTopic:       env("KAFKA_TOPIC", "crm-tasks.task.create"),
		KafkaDLQTopic:    env("KAFKA_DLQ_TOPIC", "crm-tasks.task.create.dlq"),
		KafkaGroupID:     env("KAFKA_GROUP_ID", "crm-tasks"),
		KafkaEventsTopic: env("KAFKA_EVENTS_TOPIC", "crm-tasks.task.events"),
		SeedDemo:         env("SEED_DEMO", "true") == "true",
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
