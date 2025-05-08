package functions

import (
	"errors"
	"fmt"
	"os"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsclient "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func NewNATSServer() (conn *natsclient.Conn, js jetstream.JetStream, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "nats_test")
	if err != nil {
		err = fmt.Errorf("failed to create temp directory for NATS storage: %w", err)
		return
	}
	server, err := natsserver.NewServer(&natsserver.Options{
		JetStream: true,
		StoreDir:  tmp,
		Websocket: natsserver.WebsocketOpts{
			Host:        "localhost",
			Port:        8080,
			NoTLS:       true,
			Compression: true,
		},
	})
	if err != nil {
		err = fmt.Errorf("failed to create NATS server: %w", err)
		return
	}
	// Add logs to stdout.
	server.ConfigureLogger()
	server.Start()
	cleanup = func() {
		server.Shutdown()
		os.RemoveAll(tmp)
	}

	if !server.ReadyForConnections(5 * time.Second) {
		err = errors.New("failed to start server after 5 seconds")
		return
	}

	// Create a connection.
	conn, err = natsclient.Connect("ws://localhost:8080")
	if err != nil {
		err = fmt.Errorf("failed to connect to server: %w", err)
		return
	}

	// Create a JetStream client.
	js, err = jetstream.New(conn)
	if err != nil {
		err = fmt.Errorf("failed to create jetstream: %w", err)
		return
	}

	return
}
