/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
)

func CheckHeartbeat(client *mqtt.Client, topic string, message string, lc logger.LoggingClient) {
	lc.Debug("Searching for connection...")

	c := *client
	// Stay here till connection is reestablished
	for !c.IsConnectionOpen() {
	}
	go sendHeartbeat(client, topic, message, lc)
	lc.Info("Checking heartbeat")
	var retry = false
	c.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		if string(msg.Payload()) == message {
			client.Unsubscribe(topic)
			retry = true
			return
		}
	})
	for !retry {
	}
	lc.Info("Got heartbeat, before continue, will wait 5 min...")
	// Give an extra few seconds to be sure
	time.Sleep(5 * time.Minute)
	lc.Info("Time is up, will publish now.")
}

func sendHeartbeat(client *mqtt.Client, topic string, message string, lc logger.LoggingClient) {
	lc.Info("Sending heartbeat...")
	ticker := time.NewTicker(500 * time.Millisecond)
	c := *client
	go func() {
		for range ticker.C {
			c.Publish(topic, 1, false, message)
		}
	}()
	time.Sleep(1 * time.Minute)
	ticker.Stop()
}
