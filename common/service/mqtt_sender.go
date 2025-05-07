/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"strings"
	"sync"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	gometrics "github.com/rcrowley/go-metrics"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/secure"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
)

const (
	MqttExportSizeName   = "MqttExportSize"
	MqttExportErrorsName = "MqttExportErrors"
	MetricsReservoirSize = 1028
	CorrelationHeader    = "X-Correlation-ID"

	// max concurrent MQTT publish command
	maxConcurrency = 5
)

type MqttSender interface {
	MQTTSend(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{})
	InitializeMQTTClient(ctx interfaces.AppFunctionContext) error
	ConnectToBroker(ctx interfaces.AppFunctionContext, exportData []byte) error
	GetClient() MQTT.Client
	GetMqttSizeMetrics() gometrics.Histogram
	SetMqttSizeMetrics(prevSizeMetrics gometrics.Histogram)
	GetMQTTSecretConfig() transforms.MQTTSecretConfig
	SetPersistOnError(updatedPersistOnErrorValue bool)
	GetPersistOnError() bool
}

// MQTTSecretSender ...
type MQTTSecretSender struct {
	lock                 sync.Mutex
	Client               MQTT.Client
	mqttConfig           transforms.MQTTSecretConfig
	PersistOnError       bool
	opts                 *MQTT.ClientOptions
	secretsLastRetrieved time.Time
	topicFormatter       StringValuesFormatter
	mqttSizeMetrics      gometrics.Histogram
	mqttErrorMetric      gometrics.Counter
	// used to enforce concurrency limit
	sem chan bool
}

// NewMQTTSecretSender ...
func NewMQTTSecretSender(mqttConfig transforms.MQTTSecretConfig, persistOnError bool) MqttSender {
	opts := MQTT.NewClientOptions()

	opts.AddBroker(mqttConfig.BrokerAddress)
	opts.SetClientID(mqttConfig.ClientId)
	opts.SetAutoReconnect(mqttConfig.AutoReconnect)

	//avoid casing issues
	mqttConfig.AuthMode = strings.ToLower(mqttConfig.AuthMode)
	sender := &MQTTSecretSender{
		Client:         nil,
		mqttConfig:     mqttConfig,
		PersistOnError: persistOnError,
		opts:           opts,
		sem:            make(chan bool, maxConcurrency),
	}

	return sender
}

func (sender *MQTTSecretSender) InitializeMQTTClient(ctx interfaces.AppFunctionContext) error {
	connectionInProgressLock := sender.lock.TryLock()
	if !connectionInProgressLock {
		return errors.New("earlier connection to mqtt broker in progress")
	} else {
		defer sender.lock.Unlock()
	}

	// If the conditions changed while waiting for the lock, i.e. other thread completed the initialization,
	// then skip doing anything
	secretProvider := ctx.SecretProvider()
	if sender.Client != nil && !sender.secretsLastRetrieved.Before(secretProvider.SecretsLastUpdated()) {
		return nil
	}

	ctx.LoggingClient().Info("Initializing MQTT Client")

	config := sender.mqttConfig
	mqttFactory := secure.NewMqttFactory(ctx.SecretProvider(), ctx.LoggingClient(), config.AuthMode, config.SecretName, config.SkipCertVerify)

	if len(sender.mqttConfig.KeepAlive) > 0 {
		keepAlive, err := time.ParseDuration(sender.mqttConfig.KeepAlive)
		if err != nil {
			return fmt.Errorf("in pipeline '%s', unable to parse KeepAlive value of '%s': %s", ctx.PipelineId(), sender.mqttConfig.KeepAlive, err.Error())
		}

		sender.opts.SetKeepAlive(keepAlive)
	}

	if len(sender.mqttConfig.ConnectTimeout) > 0 {
		timeout, err := time.ParseDuration(sender.mqttConfig.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("in pipeline '%s', unable to parse ConnectTimeout value of '%s': %s", ctx.PipelineId(), sender.mqttConfig.ConnectTimeout, err.Error())
		}

		sender.opts.SetConnectTimeout(timeout)
	}

	if config.Will.Enabled {
		sender.opts.SetWill(config.Will.Topic, config.Will.Payload, config.Will.Qos, config.Will.Retained)
		ctx.LoggingClient().Infof("Last Will options set for MQTT Export: %+v", config.Will)
	}

	client, err := mqttFactory.Create(sender.opts)
	if err != nil {
		return fmt.Errorf("in pipeline '%s', unable to create MQTT Client: %s", ctx.PipelineId(), err.Error())
	}

	sender.Client = client
	sender.secretsLastRetrieved = time.Now()

	return nil
}

func (sender *MQTTSecretSender) ConnectToBroker(ctx interfaces.AppFunctionContext, exportData []byte) error {
	// limit concurrent publish operations
	sender.sem <- true
	defer func() { <-sender.sem }()

	connectionInProgressLock := sender.lock.TryLock()
	if !connectionInProgressLock {
		sender.setRetryData(ctx, exportData)
		subMessage := "dropping event"
		if sender.PersistOnError {
			subMessage = "persisting Event for later retry"
		}
		return fmt.Errorf("earlier connection to mqtt broker in progress, %s", subMessage)
	} else {
		defer sender.lock.Unlock()
	}

	// If other thread made the connection while this one was waiting for the lock
	// then skip trying to connect
	if sender.Client.IsConnected() && sender.Client.IsConnectionOpen() {
		return nil
	}

	ctx.LoggingClient().Info("Connecting to mqtt server for export")
	if token := sender.Client.Connect(); token.Wait() && token.Error() != nil {
		sender.setRetryData(ctx, exportData)
		subMessage := "dropping event"
		if sender.PersistOnError {
			subMessage = "persisting Event for later retry"
		}
		return fmt.Errorf("in pipeline '%s', could not connect to mqtt server for export, %s. Error: %s", ctx.PipelineId(), subMessage, token.Error().Error())
	}
	ctx.LoggingClient().Infof("Connected to mqtt server for export in pipeline '%s'", ctx.PipelineId())
	return nil
}

// MQTTSend sends data from the previous function to the specified MQTT broker.
// If no previous function exists, then the event that triggered the pipeline will be used.
func (sender *MQTTSecretSender) MQTTSend(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	if data == nil {
		// We didn't receive a result
		return false, fmt.Errorf("function MQTTSend in pipeline '%s': No Data Received", ctx.PipelineId())
	}

	exportData, err := util.CoerceType(data)
	if err != nil {
		return false, err
	}
	// if we haven't initialized the client yet OR the cache has been invalidated (due to new/updated secrets) we need to (re)initialize the client
	if sender.Client == nil || sender.secretsLastRetrieved.Before(ctx.SecretProvider().SecretsLastUpdated()) {
		err := sender.InitializeMQTTClient(ctx)
		if err != nil {
			sender.setRetryData(ctx, exportData)
			subMessage := "dropping event"
			if sender.PersistOnError {
				subMessage = "persisting Event for later retry"
			}
			return false, fmt.Errorf("error while initializing MQTT client: %s, %s", subMessage, err.Error())
		}
	}

	publishTopic, err := sender.topicFormatter.invoke(sender.mqttConfig.Topic, ctx, data)
	if err != nil {
		return false, fmt.Errorf("in pipeline '%s', MQTT topic formatting failed: %s", ctx.PipelineId(), err.Error())
	}

	tagValue := fmt.Sprintf("%s/%s", sender.mqttConfig.BrokerAddress, publishTopic)
	tag := map[string]string{"address/topic": tagValue}

	createRegisterMetric(ctx,
		func() string { return fmt.Sprintf("%s-%s", MqttExportErrorsName, tagValue) },
		func() any { return sender.mqttErrorMetric },
		func() { sender.mqttErrorMetric = gometrics.NewCounter() },
		tag)

	createRegisterMetric(ctx,
		func() string { return fmt.Sprintf("%s-%s", MqttExportSizeName, tagValue) },
		func() any { return sender.mqttSizeMetrics },
		func() {
			sender.mqttSizeMetrics = gometrics.NewHistogram(gometrics.NewUniformSample(MetricsReservoirSize))
		},
		tag)

	if !sender.Client.IsConnected() {
		err := sender.ConnectToBroker(ctx, exportData)
		if err != nil {
			sender.mqttErrorMetric.Inc(1)
			return false, err
		}
	}

	// limit concurrent publish operations
	sender.sem <- true
	defer func() { <-sender.sem }()

	if !sender.Client.IsConnectionOpen() {
		sender.mqttErrorMetric.Inc(1)
		sender.setRetryData(ctx, exportData)
		subMessage := "dropping event"
		if sender.PersistOnError {
			subMessage = "persisting Event for later retry"
		}
		return false, fmt.Errorf("in pipeline '%s', connection to mqtt server for export not open, %s", ctx.PipelineId(), subMessage)
	}

	token := sender.Client.Publish(publishTopic, sender.mqttConfig.QoS, sender.mqttConfig.Retain, exportData)
	token.Wait()
	if token.Error() != nil {
		sender.mqttErrorMetric.Inc(1)
		sender.setRetryData(ctx, exportData)
		return false, token.Error()
	}

	// capture the size for metrics
	exportDataBytes := len(exportData)
	sender.mqttSizeMetrics.Update(int64(exportDataBytes))

	ctx.LoggingClient().Debugf("Sent %d bytes of data to MQTT Broker in pipeline '%s'", exportDataBytes, ctx.PipelineId())
	ctx.LoggingClient().Tracef("Data exported", "Transport", "MQTT", "pipeline", ctx.PipelineId(), CorrelationHeader, ctx.CorrelationID())

	return true, nil
}

func (sender *MQTTSecretSender) setRetryData(ctx interfaces.AppFunctionContext, exportData []byte) {
	// removed special handling of SAF for data-enrichment
	if sender.PersistOnError {
		ctx.SetRetryData(exportData)
	}
}

func (sender *MQTTSecretSender) GetClient() MQTT.Client {
	return sender.Client
}

func (sender *MQTTSecretSender) GetPersistOnError() bool {
	return sender.PersistOnError
}

func (sender *MQTTSecretSender) SetPersistOnError(updatedPersistOnErrorValue bool) {
	sender.PersistOnError = updatedPersistOnErrorValue
}

func (sender *MQTTSecretSender) GetMqttSizeMetrics() gometrics.Histogram {
	return sender.mqttSizeMetrics
}

func (sender *MQTTSecretSender) SetMqttSizeMetrics(prevSizeMetrics gometrics.Histogram) {
	sender.mqttSizeMetrics = prevSizeMetrics
}

func (sender *MQTTSecretSender) GetMQTTSecretConfig() transforms.MQTTSecretConfig {
	return sender.mqttConfig
}

// StringValuesFormatter defines a function signature to perform string formatting operations using an AppFunction payload.
type StringValuesFormatter func(string, interfaces.AppFunctionContext, interface{}) (string, error)

// invoke will attempt to invoke the underlying function, returning the result of ctx.ApplyValues(format) if nil
func (f StringValuesFormatter) invoke(format string, ctx interfaces.AppFunctionContext, data interface{}) (string, error) {
	if f == nil {
		return ctx.ApplyValues(format)
	} else {
		return f(format, ctx, data)
	}
}

func createRegisterMetric(ctx interfaces.AppFunctionContext,
	fullNameFunc func() string, getMetric func() any, setMetric func(),
	tags map[string]string) {
	// Only need to create and register the metric if it hasn't been created yet.
	if getMetric() == nil {
		lc := ctx.LoggingClient()
		var err error
		fullName := fullNameFunc()
		lc.Debugf("Initializing metric %s.", fullName)
		setMetric()
		metricsManger := ctx.MetricsManager()
		if metricsManger != nil {
			err = metricsManger.Register(fullName, getMetric(), tags)
		} else {
			err = errors.New("metrics manager not available")
		}

		if err != nil {
			lc.Errorf("Unable to register metric %s. Collection will continue, but metric will not be reported: %s", fullName, err.Error())
			return
		}

		lc.Infof("%s metric has been registered and will be reported (if enabled)", fullName)
	}
}
