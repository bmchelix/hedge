/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/app-services/nats-proxy/functions"
	"hedge/app-services/nats-proxy/router"
	"hedge/common/client"
	"hedge/common/config"
	"context"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"net"
	"os"
	"strings"
	"time"
)

var currentNatsServerInstanceIP string

func main() {
	service, ok := pkg.NewAppService(client.HedgeNatsProxyServiceKey)
	if !ok {
		fmt.Errorf("Failed to start App Service: %s\n", client.HedgeNatsProxyServiceKey)
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	// Get Configs
	natsServer, err := service.GetAppSetting("EdgexNatsServer")
	lc.Infof("EdgexNatsServer: %v", natsServer)

	if err != nil {
		lc.Errorf("Getting NodeType failed: %v\n", err)
		os.Exit(-1)
	}
	streamName, err := service.GetAppSetting("StreamName")
	if err != nil {
		lc.Errorf("Getting NodeType failed: %v\n", err)
		os.Exit(-1)
	}
	node, err := config.GetNode(service, "current")
	if err != nil {
		lc.Errorf("Error getting current node from hedge-admin: error: %v", err)
		os.Exit(-1)
	}

	// Initialize customDialer with a timeout
	cd := &customDialer{
		dialer: &net.Dialer{
			Timeout: 5 * time.Second,
		},
	}

	// Connect to NATS server
	// The NATS client will attempt to connect to the first available server in the list and will handle reconnections and failovers
	// if the connected server becomes unavailable
	nc, err := nats.Connect(natsServer,
		nats.ClientCert("/etc/ssl/certs/nats.crt.pem", "/etc/ssl/certs/nats.key.pem"),
		nats.Compression(true),
		nats.Timeout(10*time.Second),
		nats.SetCustomDialer(cd),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			lc.Infof("Disconnected from NATS server IP: %v due to server instance unavailability, err: %s. Attempting to reconnect to an available instance...", currentNatsServerInstanceIP, err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			lc.Infof("Reconnected to available NATS server instance, IP: %s", currentNatsServerInstanceIP)
		}),
	)

	if err != nil {
		lc.Errorf("Error connecting to NATS server: %v", err)
		return
	}
	lc.Infof("Connected to NATS Server: %s, IP: %s", natsServer, currentNatsServerInstanceIP)
	defer nc.Close()

	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		lc.Errorf("Error creating stream: %v", err)
		return
	}

	// Create NATS stream to publish and read
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        streamName,
		Description: "Stream for core-node communication",
		Subjects:    []string{streamName + ".>"},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.MemoryStorage,
		NoAck:       false,
		Compression: jetstream.S2Compression,
		AllowDirect: true,
	})
	if err != nil {
		lc.Errorf("Error creating stream: %v", err)
		return
	}

	natsProxy := &functions.NATSProxy{
		Lc:     service.LoggingClient(),
		Stream: streamName,
		Server: natsServer,
	}

	natsProxy.SubsInit(nc, node.NodeId)

	// Setup HTTP server
	router := router.NewRouter(service)
	proxy := functions.NewHTTPProxy(nc, lc, streamName, natsServer)
	router.LoadRoute(proxy)

	//Start service
	err = service.Run()
	if err != nil {
		lc.Error("Run returned error: ", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// customDialer implements the nats.CustomDialer interface
type customDialer struct {
	dialer *net.Dialer
}

// Dial creates a client connection to the given network address using net.Dialer.
func (cd *customDialer) Dial(network, address string) (net.Conn, error) {
	conn, err := cd.dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}
	// log the IP address to which the connection was established
	natsServerInstanceIP := strings.Split(conn.RemoteAddr().String(), ":")[0]
	currentNatsServerInstanceIP = natsServerInstanceIP
	return conn, nil
}
