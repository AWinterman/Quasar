package internal

import (
	"context"
	"errors"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal/configuration"
	"github.com/AWinterman/quasar/internal/queue"

	"github.com/google/uuid"
	"io"
	"time"
)

type QuasarGrpc struct {
	quasar.UnimplementedQuasarServer

	QueueService queue.QueueService
}

func NewQuasarGrpc(conf *configuration.Config) *QuasarGrpc {
	q, err := queue.NewQueue(conf)
	if err != nil {
		panic(err)
	}
	return &QuasarGrpc{QueueService: q}
}

func (s *QuasarGrpc) Enqueue(c context.Context, r *quasar.EnqueueRequest) (*quasar.Envelope, error) {
	envelope := &quasar.Envelope{
		Source:           r.Source,
		Queue:            r.Queue,
		Type:             r.Type,
		Payload:          r.Payload,
		InjectionMillis: time.Now().UnixMilli(),
		EventId: uuid.New().String(),
	}

	taskInfo, err := s.QueueService.Enqueue(c, envelope)
	if err != nil {
		return nil, err
	}
	return taskInfo, nil
}

func (s *QuasarGrpc) Subscribe(srv quasar.Quasar_SubscribeServer) error {
	req, err := srv.Recv()
	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	events, acks, errs := s.QueueService.Subscribe(srv.Context(), req.QueueName, req.Source)

	for {
		select {
		case envelope := <-events:
			if err != nil {
				return err
			}

			err = srv.Send(envelope)

			if err != nil {
				return err
			}
		case e := <-errs:
			return e
		case <-srv.Context().Done():
			return srv.Context().Err()
		}

		done, ackError := s.awaitAck(srv, acks, req.QueueName)
		if done {
			return ackError
		}
	}

}

func (s *QuasarGrpc) awaitAck(srv quasar.Quasar_SubscribeServer, acks chan bool, queueName string) (bool, error) {
	for {
		req, err := srv.Recv()
		if err == io.EOF {
			return true, nil
		}

		if err != nil {
			return true, err
		}

		if queueName != "" && req.QueueName != queueName {
			return true, errors.New("mismatched non empty queue name")
		}

		if req.Ack.Number() != quasar.AckResponse_UNKNOWN_ACK_RESPONSE.Number() {
			acks <- req.Ack.Number() == quasar.AckResponse_ACK.Number()
			return false, nil
		}
	}
}
