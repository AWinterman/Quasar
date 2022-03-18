package main

import (
	"context"
	quasar "github.com/AWinterman/quasar/gen/go"
	"github.com/AWinterman/quasar/internal"
	"github.com/AWinterman/quasar/internal/configuration"

	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
	"log"
	"net"
	"os"
	"os/signal"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalln("Exiting with error", err)
	}
}

func run() error {
	ctx := context.Background()
	notifyContext, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	conf := &configuration.Config{}

	s := os.Args[0]
	file, err3 := os.ReadFile(s)
	if err3 != nil {
		return err3
	}

	err := yaml.Unmarshal(file, conf)

	if err != nil {
		return err
	}

	server, err := buildServer(conf)

	if err != nil {
		return err
	}

	lis, err2 := contextListener(notifyContext, conf)
	if err2 != nil {
		return err2
	}

	// todo: graceful shutdown
	return server.Serve(lis)
}

func contextListener(ctx context.Context, conf *configuration.Config) (net.Listener, error) {
	listenConfig := net.ListenConfig{}
	lis, err := listenConfig.Listen(ctx, "tcp4", conf.GRPC.Addr)

	if err != nil {
		return nil, err
	}
	return lis, nil
}

func buildServer(conf *configuration.Config) (*grpc.Server, error) {

	srv := internal.NewQuasarGrpc(conf)
	server := grpc.NewServer()
	quasar.RegisterQuasarServer(server, srv)
	return server, nil
}

