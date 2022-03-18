package queue

import (
	"context"
	"errors"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal/configuration"

	"github.com/hibiken/asynq"
)

type ChannelHandler struct {
	Tasks  chan *quasar.Envelope
	Ack    chan bool
	Source string
}

func NewChannelHandler(source string) *ChannelHandler {
	return &ChannelHandler{
		Tasks: make(chan *quasar.Envelope, 0),
		Ack:   make(chan bool, 0),
		Source: source,
	}
}

func (queue *ChannelHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	envelope, err := unmarshal(task.Payload())
	if err != nil {
		return err
	}
	select {
	case queue.Tasks <- envelope:
		// then we wait for them to respond
		select {
		case ack := <-queue.Ack:
			if ack {
				return nil
			} else {
				return errors.New("consumer nacked the message; returning an error so it is retried")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

type queueService struct {
	opts   asynq.RedisClientOpt
	client *asynq.Client
}

func RedisQueueService(config *configuration.Config) QueueService {
	opts := asynq.RedisClientOpt{Addr: config.Redis.Addrs[0]}
	return &queueService{
		opts: opts, client: asynq.NewClient(opts),
	}
}

func (s *queueService) Subscribe(cxt context.Context, queue string, source string) (chan *quasar.Envelope, chan bool, chan error) {
	consumer := s.subscriberFactory(cxt, queue)

	handler := NewChannelHandler(source)
	errs := make(chan error)

	go func() {
		err := consumer.Run(handler)
		defer consumer.Shutdown()
		if err != nil {
			errs <- err
		}
	}()
	return handler.Tasks, handler.Ack, errs
}

func (s *queueService) subscriberFactory(cxt context.Context, queue string) *asynq.Server {
	consumer := asynq.NewServer(s.opts, asynq.Config{
		Concurrency: 1,
		Queues:      map[string]int{queue: 1},
		BaseContext: func() context.Context {
			return cxt
		},
	})
	return consumer
}

func (s *queueService) Enqueue(ctx context.Context, envelope *quasar.Envelope) (*quasar.Envelope, error) {
	bytes, err := marshal(envelope)
	if err != nil {
		return nil, err
	}
	enqueued, err := s.client.EnqueueContext(
		ctx,
		asynq.NewTask(
			envelope.Type,
			bytes,
			asynq.Queue(envelope.Queue),
			asynq.TaskID(envelope.EventId),
		),
	)

	response := &quasar.Envelope{
		Source:          envelope.Source,
		Queue:           enqueued.Queue,
		Type:            enqueued.Type,
		Payload:         enqueued.Payload,
		InjectionMillis: envelope.InjectionMillis,
		EventId:         enqueued.ID,
	}

	return response, err
}
