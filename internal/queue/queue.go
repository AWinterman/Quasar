package queue

import (
	"context"
	"errors"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal/configuration"

	"google.golang.org/protobuf/proto"
)

func marshal(envelope *quasar.Envelope) ([]byte, error) {
	return proto.Marshal(envelope)
}


func unmarshal(b []byte) (*quasar.Envelope, error) {
	envelope := &quasar.Envelope{}
	err := proto.Unmarshal(b, envelope)
	if err != nil {
		return nil, err
	}
	return envelope, nil
}

type QueueService interface {
	Subscribe(cxt context.Context, queue string, source string) (chan *quasar.Envelope, chan bool, chan error)
	Enqueue(ctx context.Context, envelope *quasar.Envelope) (*quasar.Envelope, error)
}

func NewQueue(conf *configuration.Config) (QueueService, error) {
	if conf.Redis != nil {
		return RedisQueueService(conf), nil

	} else if conf.Nats != nil {
		return NatsQueueService(conf.Nats)
	} else {
		return nil, errors.New("no queue backend configured")
	}
}

