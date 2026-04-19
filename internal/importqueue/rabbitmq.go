package importqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"

	"eino_agent/internal/config"
)

// Task 表示一个异步导入任务。
type Task struct {
	KnowledgeID      string `json:"knowledge_id"`
	KnowledgeBaseID  string `json:"knowledge_base_id"`
	SourceType       string `json:"source_type"`
	FilePath         string `json:"file_path,omitempty"`
	FileName         string `json:"file_name,omitempty"`
	FileType         string `json:"file_type,omitempty"`
	Title            string `json:"title,omitempty"`
	SourceURL        string `json:"source_url,omitempty"`
	EnableMultimodal bool   `json:"enable_multimodal,omitempty"`
}

// Queue 定义导入队列的基本能力。
type Queue interface {
	Enqueue(ctx context.Context, task Task) error
	StartConsumer(ctx context.Context, handler func(context.Context, Task) error) error
	Close() error
}

// RabbitMQQueue 是 RabbitMQ 导入队列实现。
type RabbitMQQueue struct {
	conn      *amqp.Connection
	publishCh *amqp.Channel
	consumeCh *amqp.Channel
	queueName string
	tag       string
}

// NewRabbitMQQueue 初始化 RabbitMQ 队列。
func NewRabbitMQQueue(cfg config.ImportQueueConfig) (*RabbitMQQueue, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("connect rabbitmq: %w", err)
	}

	publishCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("create publish channel: %w", err)
	}

	consumeCh, err := conn.Channel()
	if err != nil {
		_ = publishCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("create consume channel: %w", err)
	}

	if err := consumeCh.Qos(cfg.PrefetchCount, 0, false); err != nil {
		_ = consumeCh.Close()
		_ = publishCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("set qos: %w", err)
	}

	if _, err := publishCh.QueueDeclare(
		cfg.QueueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = consumeCh.Close()
		_ = publishCh.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declare queue: %w", err)
	}

	return &RabbitMQQueue{
		conn:      conn,
		publishCh: publishCh,
		consumeCh: consumeCh,
		queueName: cfg.QueueName,
		tag:       cfg.ConsumerTag,
	}, nil
}

// Enqueue 发布导入任务。
func (q *RabbitMQQueue) Enqueue(ctx context.Context, task Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	return q.publishCh.PublishWithContext(ctx,
		"",
		q.queueName,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// StartConsumer 启动后台消费者。
func (q *RabbitMQQueue) StartConsumer(ctx context.Context, handler func(context.Context, Task) error) error {
	deliveries, err := q.consumeCh.Consume(
		q.queueName,
		q.tag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-deliveries:
				if !ok {
					return
				}

				var task Task
				if err := json.Unmarshal(msg.Body, &task); err != nil {
					log.Printf("[ImportQueue] invalid task payload: %v", err)
					_ = msg.Nack(false, false)
					continue
				}

				if err := handler(ctx, task); err != nil {
					log.Printf("[ImportQueue] task failed: knowledge_id=%s err=%v", task.KnowledgeID, err)
					_ = msg.Nack(false, false)
					continue
				}

				_ = msg.Ack(false)
			}
		}
	}()

	return nil
}

// Close 关闭 RabbitMQ 连接。
func (q *RabbitMQQueue) Close() error {
	if q.consumeCh != nil {
		_ = q.consumeCh.Close()
	}
	if q.publishCh != nil {
		_ = q.publishCh.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
