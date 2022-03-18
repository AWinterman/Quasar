package main

import (
	"context"
	"fmt"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal/configuration"
	"github.com/docker/go-connections/nat"
	"google.golang.org/grpc"
	"sort"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type redisContainer struct {
	testcontainers.Container
	URI string
}

func setupRedis(ctx context.Context) (*redisContainer, error) {
	port, err := nat.NewPort("tcp", "6379")
	if err != nil {
		return nil, err
	}
	req := testcontainers.ContainerRequest{
		Image:        "redis",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort(port),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("%s:%s", ip, mappedPort.Port())

	return &redisContainer{Container: container, URI: uri}, nil
}

func TestClientServerInteraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx, stop := context.WithTimeout(context.Background(), 30 * time.Second)
	defer stop()

	redis, err := setupRedis(ctx)
	if err != nil {
		if err != nil {
			t.Fatal(err)
		}
	}

	config := &configuration.Config{
		GRPC: configuration.GRPC{
			Addr: "0.0.0.0:0",
		},
		Redis: &configuration.Redis{
			Addrs: []string{redis.URI},
		},
	}

	defer func(redis *redisContainer, ctx context.Context) {
		err := redis.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
	}(redis, ctx)

	server, addr := createAndStartServer(t, config, ctx)
	defer server.GracefulStop()
	err, client := newClient(t, addr)

	t.Run("Subscribe Enqueue Recv Ack Recv Ack", func(t *testing.T) {
		subscriber, err := client.Subscribe(ctx)
		if err != nil {
			t.Fatal(err)
		}

		request := quasar.SubscribeRequest{
			QueueName: "quasar",
			Ack:       nil,
		}

		err = subscriber.Send(&request)

		if err != nil {
			return
		}

		one, err := client.Enqueue(ctx, &quasar.EnqueueRequest{
			Queue:   "quasar",
			Type:    "testing",
			Payload: []byte("1"),
			Source:  "test",
		})

		if err != nil {
			t.Fatal(err)
		}

		two, err := client.Enqueue(ctx, &quasar.EnqueueRequest{
			Queue:   "quasar",
			Type:    "testing",
			Payload: []byte("1"),
		})

		if err != nil {
			t.Fatal(err)
		}

		recvOne, err := subscriber.Recv()
		if err != nil {
			t.Fatal(err)
		}

		request.Ack = quasar.AckResponse_ACK.Enum()
		err = subscriber.Send(&request)
		if err != nil {
			t.Fatal(err)
		}

		recvTwo, err := subscriber.Recv()
		if err != nil {
			t.Fatal(err)
		}

		recvs := []string{recvOne.EventId, recvTwo.EventId}
		sort.Strings(recvs)
		sends := []string{one.EventId, two.EventId}
		sort.Strings(sends)

		if recvs[0] != sends[0] {
			t.Errorf("0: %v != %v", recvs[0], sends[0])
		}
		if recvs[1] != sends[1] {
			t.Errorf("0: %v != %v", recvs[1], sends[1])
		}

		err = subscriber.CloseSend()
		if err != nil {
			t.Error(err)
		}

	})

}

func newClient(t *testing.T, addr string) (error, quasar.QuasarClient) {
	dial, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	client := quasar.NewQuasarClient(dial)
	return err, client
}

func createAndStartServer(t *testing.T, config *configuration.Config, ctx context.Context) (*grpc.Server, string) {

	t.Log("Found config ", *config)

	server, err := buildServer(config)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := contextListener(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	serverAddress := listener.Addr().String()

	go func() {
		err = server.Serve(listener)
		if err != nil {
			t.Error(err)
		}
	}()
	return server, serverAddress
}
