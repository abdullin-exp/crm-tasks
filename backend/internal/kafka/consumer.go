// Package kafka — создание задач по событиям из Kafka.
//
// Формат сообщения (JSON), топик по умолчанию crm-tasks.task.create:
//
//	{
//	  "event_id": "уникальный id события (идемпотентность)",
//	  "type_code": "email_order",           // или "task_type_id": 1
//	  "title": "Клиент разместил заказ через почту",
//	  "description": "...",
//	  "priority": 2,
//	  "executor_user_id": "uuid из ref_user",
//	  "contractor_id": "uuid из ref_contractor",
//	  "contact_contractor_id": "uuid",
//	  "solve_plan_at": "2026-11-25T18:00:00+03:00",
//	  "legacy_task_id": "uuid задачи doc_task (если событие из CRM)",
//	  "data": { "<код поля>": <значение> }
//	}
//
// Ошибки валидации и некорректный JSON отправляются в DLQ-топик с описанием ошибки,
// временные ошибки (БД недоступна) повторяются без коммита offset.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	kgo "github.com/segmentio/kafka-go"

	"crm-tasks/internal/form"
	"crm-tasks/internal/service"
)

type Message struct {
	service.TaskInput
	EventID string `json:"event_id"`
}

type Consumer struct {
	Brokers  []string
	Topic    string
	DLQTopic string
	GroupID  string
	Service  *service.Service

	mu        sync.Mutex
	enabled   bool
	lastError string
	errorAt   time.Time
	lastAt    *time.Time
	processed int64
	failed    int64
}

func (c *Consumer) Status() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	lastErr := c.lastError
	if time.Since(c.errorAt) > 2*time.Minute {
		lastErr = "" // старые транзиентные ошибки (например, при старте брокера) не показываем
	}
	return map[string]any{
		"enabled":         c.enabled,
		"brokers":         c.Brokers,
		"topic":           c.Topic,
		"dlq_topic":       c.DLQTopic,
		"processed":       c.processed,
		"failed":          c.failed,
		"last_error":      lastErr,
		"last_message_at": c.lastAt,
	}
}

func (c *Consumer) setError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.lastError = ""
	} else {
		c.lastError = err.Error()
		c.errorAt = time.Now()
	}
}

// Disabled — заглушка статуса, когда Kafka выключена.
type Disabled struct{}

func (Disabled) Status() map[string]any { return map[string]any{"enabled": false} }

// Run запускает чтение топика до отмены контекста.
func (c *Consumer) Run(ctx context.Context) {
	c.mu.Lock()
	c.enabled = true
	c.mu.Unlock()

	for ctx.Err() == nil {
		if err := c.ensureTopics(); err != nil {
			c.setError(fmt.Errorf("kafka недоступна: %w", err))
			slog.Warn("kafka: cannot ensure topics, retrying", "err", err)
			sleep(ctx, 10*time.Second)
			continue
		}
		break
	}

	reader := kgo.NewReader(kgo.ReaderConfig{
		Brokers:     c.Brokers,
		GroupID:     c.GroupID,
		Topic:       c.Topic,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     time.Second,
		StartOffset: kgo.FirstOffset,
		ErrorLogger: kgo.LoggerFunc(func(msg string, args ...any) {
			c.setError(fmt.Errorf(msg, args...))
		}),
	})
	defer reader.Close()
	dlq := &kgo.Writer{Addr: kgo.TCP(c.Brokers...), Topic: c.DLQTopic, Balancer: &kgo.LeastBytes{}, RequiredAcks: kgo.RequireAll, BatchTimeout: 10 * time.Millisecond}
	defer dlq.Close()

	slog.Info("kafka consumer started", "topic", c.Topic, "group", c.GroupID)
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.setError(err)
			slog.Error("kafka fetch failed", "err", err)
			sleep(ctx, 5*time.Second)
			continue
		}
		c.handle(ctx, msg, dlq)
		if ctx.Err() != nil {
			return
		}
		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			slog.Error("kafka commit failed", "err", err)
		}
	}
}

// handle обрабатывает сообщение; возвращается только когда сообщение можно коммитить.
func (c *Consumer) handle(ctx context.Context, msg kgo.Message, dlq *kgo.Writer) {
	log := slog.With("partition", msg.Partition, "offset", msg.Offset)
	var m Message
	if err := json.Unmarshal(msg.Value, &m); err != nil {
		c.toDLQ(ctx, dlq, msg, "некорректный JSON: "+err.Error())
		return
	}
	extID := m.EventID
	if extID == "" && m.ExternalID != nil {
		extID = *m.ExternalID
	}
	if extID == "" {
		// без event_id идемпотентность обеспечивается позицией сообщения в топике
		extID = msg.Topic + ":" + strconv.Itoa(msg.Partition) + ":" + strconv.FormatInt(msg.Offset, 10)
	}
	m.ExternalID = &extID

	backoff := time.Second
	for {
		t, err := c.Service.Create(ctx, m.TaskInput, service.SourceKafka)
		var verrs form.Errors
		switch {
		case err == nil:
			log.Info("kafka: task created", "task", t.Number, "event_id", extID)
			c.ok()
			return
		case errors.Is(err, service.ErrDuplicate):
			log.Info("kafka: duplicate event skipped", "event_id", extID, "task", t.Number)
			c.ok()
			return
		case errors.As(err, &verrs):
			c.toDLQ(ctx, dlq, msg, verrs.Error())
			return
		default:
			if ctx.Err() != nil {
				return
			}
			c.setError(err)
			log.Error("kafka: create failed, retrying", "err", err, "backoff", backoff)
			sleep(ctx, backoff)
			if backoff < time.Minute {
				backoff *= 2
			}
		}
	}
}

func (c *Consumer) ok() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.processed++
	c.lastAt = &now
	c.lastError = ""
}

func (c *Consumer) toDLQ(ctx context.Context, dlq *kgo.Writer, msg kgo.Message, reason string) {
	slog.Warn("kafka: message rejected", "offset", msg.Offset, "reason", reason)
	c.mu.Lock()
	c.failed++
	c.mu.Unlock()
	err := dlq.WriteMessages(ctx, kgo.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: append(msg.Headers,
			kgo.Header{Key: "error", Value: []byte(reason)},
			kgo.Header{Key: "source_offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
		),
	})
	if err != nil {
		slog.Error("kafka: write to DLQ failed", "err", err)
	}
}

func (c *Consumer) ensureTopics() error {
	return ensureTopics(c.Brokers,
		kgo.TopicConfig{Topic: c.Topic, NumPartitions: 3, ReplicationFactor: 1},
		kgo.TopicConfig{Topic: c.DLQTopic, NumPartitions: 1, ReplicationFactor: 1},
	)
}

// ensureTopics создаёт топики, если их ещё нет.
func ensureTopics(brokers []string, topics ...kgo.TopicConfig) error {
	conn, err := kgo.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	cc, err := kgo.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer cc.Close()
	err = cc.CreateTopics(topics...)
	if errors.Is(err, kgo.TopicAlreadyExists) {
		return nil
	}
	return err
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
