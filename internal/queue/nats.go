package queue

import (
	"context"
	"fmt"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal/configuration"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/encoding/prototext"
	"time"
)

type natsQueueService struct {
	nats.JetStream
}

func (n natsQueueService) Subscribe(cxt context.Context, queue string, source string) (chan *quasar.Envelope, chan bool, chan error) {
	subscribe, err := n.JetStream.PullSubscribe(queue, source, nats.Bind(queue, source))
	errors := make(chan error)
	events := make(chan *quasar.Envelope)
	acks := make(chan bool)

	if err != nil {
		close(events)
		close(acks)
		errors <- err
		return events, acks, errors
	}

	go func() {
		for {
			msgs, err := subscribe.Fetch(10, nats.ContextOpt{Context: cxt}, nats.MaxWait(time.Second*1))
			if err != nil {
				errors <- err
				break
			}

			for _, msg := range msgs {
				data := msg.Data
				envelope := &quasar.Envelope{}
				err := prototext.Unmarshal(data, envelope)
				if err != nil {
					errors <- err
					break
				}

				select {
				// send the event to the channel
				case events <- envelope:
					select {
					// wait for the acks signal from the client
					case doAck := <-acks:
						err := n.acknowledge(cxt, doAck, msg)
						if err != nil {
							errors <- err
						}
					// unless we get cancelled or run out of time
					case <-cxt.Done():
						errors <- cxt.Err()
					}
					// unless we get cancelled or run out of time
				case <-cxt.Done():
					errors <- cxt.Err()
				}

			}
		}
	}()

	return events, acks, errors
}

func (n natsQueueService) acknowledge(ctx context.Context, doAck bool, msg *nats.Msg) error {
	if doAck {
		return msg.Ack(nats.ContextOpt{Context: ctx})
	} else {
		return msg.Nak(nats.ContextOpt{Context: ctx})

	}
}

func (n natsQueueService) Enqueue(ctx context.Context, envelope *quasar.Envelope) (*quasar.Envelope, error) {
	bytes, err := prototext.Marshal(envelope)
	if err != nil {
		return nil, err
	}

	_, err = n.JetStream.Publish(
		fmt.Sprintf("%s.%s", envelope.Queue, envelope.Type),
		bytes,
		nats.ContextOpt{Context: ctx},
	)

	if err != nil {
		return nil, err
	}

	return envelope, nil
}

func NatsQueueService(config *configuration.NATS) (QueueService, error) {
	con, err := nats.Connect(config.Addr)

	if err != nil {
		return nil, err
	}
	js, err := con.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return nil, err
	}
	return natsQueueService{js}, nil
}
