package redis

import "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

var loggingClient logger.LoggingClient

func newLoggingClient() logger.LoggingClient {
	if loggingClient == nil {
		return logger.NewClient("redis", "INFO")
	}
	return loggingClient
}
