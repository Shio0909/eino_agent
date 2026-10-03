package importqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

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
	ReplaceExisting  bool   `json:"replace_existing,omitempty"`
	// SourceHash is the hash of the uploaded file this task was created for. If
	// the document's stored hash has moved on when the task runs, a newer upload
	// has replaced it and this task stands down.
	SourceHash string `json:"source_hash,omitempty"`
	// Attempt 从 1 开始，重试时递增。
	Attempt int `json:"attempt,omitempty"`
}

// Handler 处理单个任务。返回 Permanent 包装的错误表示不可重试，直接进入死信队列。
type Handler func(context.Context, Task) error

// Queue 定义导入队列的基本能力。
type Queue interface {
	Enqueue(ctx context.Context, task Task) error
	StartConsumer(ctx context.Context, handler func(context.Context, Task) error) error
	Close() error
}

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// Permanent 标记不可重试的错误（如文件不存在、格式不支持）。
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

// IsPermanent 判断错误是否不可重试。
func IsPermanent(err error) bool {
	var p permanentError
	return errors.As(err, &p)
}

// MaxAttempts 是单个任务的最大投递次数（含首次）。
const MaxAttempts = 3

const (
	reconnectMinDelay = time.Second
	reconnectMaxDelay = 30 * time.Second
)

// RabbitMQQueue 是 RabbitMQ 导入队列实现：
//   - 发布使用 publisher confirm，确认 broker 已持久化；
//   - 失败任务按 Attempt 重新发布，超过上限或永久错误写入 <queue>.dlq；
//   - 连接断开后自动重连并恢复消费。
type RabbitMQQueue struct {
	cfg         config.ImportQueueConfig
	dlqName     string
	maxAttempts int

	mu        sync.Mutex
	conn      *amqp.Connection
	publishCh *amqp.Channel
	closed    bool

	consumerOnce sync.Once
	wg           sync.WaitGroup
}

// NewRabbitMQQueue 初始化 RabbitMQ 队列。
func NewRabbitMQQueue(cfg config.ImportQueueConfig) (*RabbitMQQueue, error) {
	q := &RabbitMQQueue{
		cfg:         cfg,
		dlqName:     cfg.QueueName + ".dlq",
		maxAttempts: MaxAttempts,
	}
	if err := q.connect(); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *RabbitMQQueue) connect() error {
	conn, err := amqp.Dial(q.cfg.URL)
	if err != nil {
		return fmt.Errorf("connect rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("create publish channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = conn.Close()
		return fmt.Errorf("enable publisher confirms: %w", err)
	}
	for _, name := range []string{q.cfg.QueueName, q.dlqName} {
		if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
			_ = conn.Close()
			return fmt.Errorf("declare queue %s: %w", name, err)
		}
	}

	q.mu.Lock()
	q.conn = conn
	q.publishCh = ch
	q.mu.Unlock()
	return nil
}

func (q *RabbitMQQueue) currentConn() (*amqp.Connection, *amqp.Channel, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, nil, errors.New("import queue closed")
	}
	if q.conn == nil || q.conn.IsClosed() {
		return nil, nil, errors.New("rabbitmq connection unavailable")
	}
	return q.conn, q.publishCh, nil
}

// Enqueue 发布导入任务并等待 broker 确认。
func (q *RabbitMQQueue) Enqueue(ctx context.Context, task Task) error {
	if task.Attempt <= 0 {
		task.Attempt = 1
	}
	return q.publish(ctx, q.cfg.QueueName, task)
}

func (q *RabbitMQQueue) publish(ctx context.Context, queue string, task Task) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.publishCh == nil || q.conn == nil || q.conn.IsClosed() {
		return errors.New("rabbitmq connection unavailable")
	}
	confirm, err := q.publishCh.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish task: %w", err)
	}
	ok, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait publish confirm: %w", err)
	}
	if !ok {
		return errors.New("rabbitmq nacked task publish")
	}
	return nil
}

// StartConsumer 启动后台消费者，连接断开时自动重连。
func (q *RabbitMQQueue) StartConsumer(ctx context.Context, handler func(context.Context, Task) error) error {
	started := false
	q.consumerOnce.Do(func() {
		started = true
		q.wg.Add(1)
		go q.consumeLoop(ctx, handler)
	})
	if !started {
		return errors.New("consumer already started")
	}
	return nil
}

func (q *RabbitMQQueue) consumeLoop(ctx context.Context, handler Handler) {
	defer q.wg.Done()
	delay := reconnectMinDelay
	for {
		if ctx.Err() != nil {
			return
		}
		err := q.consumeOnce(ctx, handler)
		if ctx.Err() != nil {
			return
		}
		q.mu.Lock()
		closed := q.closed
		q.mu.Unlock()
		if closed {
			return
		}
		log.Printf("[ImportQueue] consumer stopped: %v; reconnecting in %s", err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if err := q.reconnect(); err != nil {
			log.Printf("[ImportQueue] reconnect failed: %v", err)
			delay *= 2
			if delay > reconnectMaxDelay {
				delay = reconnectMaxDelay
			}
			continue
		}
		delay = reconnectMinDelay
	}
}

func (q *RabbitMQQueue) reconnect() error {
	q.mu.Lock()
	if q.conn != nil && !q.conn.IsClosed() {
		q.mu.Unlock()
		return nil
	}
	q.mu.Unlock()
	return q.connect()
}

func (q *RabbitMQQueue) consumeOnce(ctx context.Context, handler Handler) error {
	conn, _, err := q.currentConn()
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("create consume channel: %w", err)
	}
	defer ch.Close()

	prefetch := q.cfg.PrefetchCount
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}
	deliveries, err := ch.Consume(q.cfg.QueueName, q.cfg.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume queue: %w", err)
	}
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case amqpErr := <-closeCh:
			return fmt.Errorf("connection closed: %v", amqpErr)
		case msg, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed")
			}
			q.handleDelivery(ctx, msg, handler)
		}
	}
}

func (q *RabbitMQQueue) handleDelivery(ctx context.Context, msg amqp.Delivery, handler Handler) {
	var task Task
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		log.Printf("[ImportQueue] invalid task payload, moving to DLQ: %v", err)
		q.deadLetterRaw(ctx, msg)
		return
	}
	if task.Attempt <= 0 {
		task.Attempt = 1
	}

	err := handler(ctx, task)
	if err == nil {
		_ = msg.Ack(false)
		return
	}

	if !IsPermanent(err) && task.Attempt < q.maxAttempts && ctx.Err() == nil {
		next := task
		next.Attempt++
		if pubErr := q.publish(ctx, q.cfg.QueueName, next); pubErr == nil {
			log.Printf("[ImportQueue] task failed (attempt %d/%d), requeued: knowledge_id=%s err=%v", task.Attempt, q.maxAttempts, task.KnowledgeID, err)
			_ = msg.Ack(false)
			return
		}
		// 重新发布失败时交回 broker，由其重新投递。
		_ = msg.Nack(false, true)
		return
	}

	log.Printf("[ImportQueue] task failed permanently, moving to DLQ: knowledge_id=%s attempt=%d err=%v", task.KnowledgeID, task.Attempt, err)
	if pubErr := q.publish(ctx, q.dlqName, task); pubErr != nil {
		_ = msg.Nack(false, true)
		return
	}
	_ = msg.Ack(false)
}

func (q *RabbitMQQueue) deadLetterRaw(ctx context.Context, msg amqp.Delivery) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.publishCh == nil {
		_ = msg.Nack(false, false)
		return
	}
	if err := q.publishCh.PublishWithContext(ctx, "", q.dlqName, false, false, amqp.Publishing{
		ContentType:  msg.ContentType,
		DeliveryMode: amqp.Persistent,
		Body:         msg.Body,
	}); err != nil {
		_ = msg.Nack(false, false)
		return
	}
	_ = msg.Ack(false)
}

// Close 关闭连接并等待消费者退出（调用方应先取消 StartConsumer 的 ctx）。
func (q *RabbitMQQueue) Close() error {
	q.mu.Lock()
	q.closed = true
	conn := q.conn
	ch := q.publishCh
	q.mu.Unlock()

	if ch != nil {
		_ = ch.Close()
	}
	var err error
	if conn != nil && !conn.IsClosed() {
		err = conn.Close()
	}
	q.wg.Wait()
	return err
}
