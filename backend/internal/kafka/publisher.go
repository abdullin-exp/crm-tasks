package kafka

import (
	"context"
	"log/slog"
	"sync"
	"time"

	kgo "github.com/segmentio/kafka-go"

	"crm-tasks/internal/store"
)

// Publisher отправляет события задач из outbox в Kafka.
//
// Топик по умолчанию crm-tasks.task.events, ключ — id задачи (порядок событий одной задачи сохраняется).
// Сообщение: {"event_id","event_type","occurred_at","task":{...полный снимок задачи...},"details":{...}}
// Типы: task.created, task.updated, task.closed, task.reopened, task.taken, task.transferred.
// Доставка at-least-once: потребитель должен дедуплицировать по event_id.
type Publisher struct {
	Brokers []string
	Topic   string
	Store   *store.Store

	mu        sync.Mutex
	published int64
	lastError string
	errorAt   time.Time
}

func (p *Publisher) Status() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	lastErr := p.lastError
	if time.Since(p.errorAt) > 2*time.Minute {
		lastErr = ""
	}
	pending, _ := p.Store.OutboxPending(context.Background())
	return map[string]any{"topic": p.Topic, "published": p.published, "pending": pending, "last_error": lastErr}
}

func (p *Publisher) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := ensureTopics(p.Brokers, kgo.TopicConfig{Topic: p.Topic, NumPartitions: 3, ReplicationFactor: 1}); err != nil {
			p.fail(err)
			sleep(ctx, 10*time.Second)
			continue
		}
		break
	}
	w := &kgo.Writer{
		Addr:         kgo.TCP(p.Brokers...),
		Topic:        p.Topic,
		Balancer:     &kgo.Hash{}, // один ключ — одна партиция
		RequiredAcks: kgo.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer w.Close()
	slog.Info("kafka publisher started", "topic", p.Topic)

	cleanup := time.NewTicker(time.Hour)
	defer cleanup.Stop()
	backoff := time.Second
	for ctx.Err() == nil {
		select {
		case <-cleanup.C:
			if err := p.Store.CleanupOutbox(ctx, 7*24*time.Hour); err != nil {
				slog.Warn("outbox cleanup failed", "err", err)
			}
		default:
		}
		n, err := p.Store.ProcessOutbox(ctx, 100, func(batch []store.OutboxRow) error {
			msgs := make([]kgo.Message, len(batch))
			for i, r := range batch {
				msgs[i] = kgo.Message{
					Key:   []byte(r.TaskID),
					Value: r.Payload,
					Headers: []kgo.Header{
						{Key: "event_type", Value: []byte(r.EventType)},
						{Key: "event_id", Value: []byte(r.EventID)},
					},
				}
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return w.WriteMessages(wctx, msgs...)
		})
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return
			}
			p.fail(err)
			slog.Error("outbox publish failed", "err", err, "backoff", backoff)
			sleep(ctx, backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
		case n > 0:
			p.mu.Lock()
			p.published += int64(n)
			p.lastError = ""
			p.mu.Unlock()
			backoff = time.Second
		default:
			sleep(ctx, 500*time.Millisecond)
		}
	}
}

func (p *Publisher) fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
	p.errorAt = time.Now()
}

// Combined — общий статус консьюмера и публикатора для /api/health.
type Combined struct {
	Consumer  *Consumer
	Publisher *Publisher
}

func (c Combined) Status() map[string]any {
	st := c.Consumer.Status()
	st["publisher"] = c.Publisher.Status()
	return st
}
