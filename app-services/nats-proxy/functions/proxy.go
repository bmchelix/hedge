package functions

import (
	"hedge/app-services/nats-proxy/dto"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPHandler represents the HTTP server handler interface
type HTTPHandler interface {
	HandleRequest(w http.ResponseWriter, r *http.Request)
}

// NATSClient represents the NATS client interface
type NATSClient interface {
	Publish(subject string, data []byte) error
	Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error)
}

// HTTPProxy represents the HTTP-NATS proxy
type HTTPProxy struct {
	nc     NATSClient
	lc     logger.LoggingClient
	stream string
	server string
}

// NewHTTPProxy creates a new instance of HTTPProxy
func NewHTTPProxy(nc NATSClient, lc logger.LoggingClient, str string, srv string) *HTTPProxy {
	return &HTTPProxy{nc: nc, lc: lc, stream: str, server: srv}
}

func (proxy *HTTPProxy) HandleRequest(w http.ResponseWriter, r *http.Request) {
	proxy.lc.Debug("Handling incoming request")
	// Check if request is a websocket upgrade
	isws := isWebSocket(r)
	if isws {
		// Handle websocket request
		go proxy.handleWebSocketClient(w, r)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		proxy.lc.Errorf("Error reading request body %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.URL.Scheme = "http"
	url := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	nodeId := r.Header.Get("X-NODE-ID")
	if len(url) < 1 || nodeId == "" {
		http.Error(w, "Missing required headers", http.StatusBadRequest)
		return
	}

	r.URL.Host = url[0]
	msg := dto.Request{
		ID:     nodeId,
		Method: r.Method,
		Url:    r.URL.String(),
		Header: r.Header,
		Body:   body,
		IsWs:   isws,
	}
	proxy.lc.Debugf("Got request: %v", r)

	// Publish message to NATS
	payload, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reply, err := proxy.nc.Request(msg.ID, payload, 60*time.Second)
	if err != nil {
		proxy.lc.Errorf("Error getting response from %s: %s", msg.ID, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msg.Header = map[string][]string{}
	err = json.Unmarshal(reply.Data, &msg)
	if err != nil {
		proxy.lc.Errorf("Error unmarshal reply from NATS: %v", err)
		return
	}
	for k, v := range msg.Header {
		w.Header().Set(k, v[0])
	}
	w.WriteHeader(msg.Status)
	_, err = io.Copy(w, bytes.NewReader(msg.Body))
	if err != nil {
		proxy.lc.Errorf("Error writing response %v", err)
		return
	}
}

// handleWebSocketClient handles WS connection between nats-proxy and a browser
func (proxy *HTTPProxy) handleWebSocketClient(w http.ResponseWriter, r *http.Request) {
	// Create websocket upgrader
	var wsUpgrade = websocket.Upgrader{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		WriteBufferPool:   &sync.Pool{},
		EnableCompression: true,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	topic := r.Header.Get("X-NODE-ID")

	nc, err := nats.Connect(proxy.server)
	if err != nil {
		proxy.lc.Errorf("Error connecting to NATS server: %v", err)
		return
	}
	defer nc.Close()

	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		proxy.lc.Errorf("Error connecting stream server: %v", err)
		return
	}

	// Obtain stream to publish and read
	stream, err := js.Stream(ctx, proxy.stream)
	if err != nil {
		proxy.lc.Errorf("Error creating stream: %v", err)
		return
	}

	// Create core consumer to read stream
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "core_consumer_" + topic,
		FilterSubject: proxy.stream + ".core." + topic,
	})
	if err != nil {
		proxy.lc.Errorf("Error creating consumer: %v", err)
		return
	}

	// Upgrade the connection to WebSocket
	conn, err := wsUpgrade.Upgrade(w, r, nil)
	if err != nil {
		proxy.lc.Errorf("Error upgrading to websocket connection: %v", err)
		http.Error(w, "Error upgrading to WebSocket connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Consume to NATS message
	consumeContext, err := consumer.Consume(func(msg jetstream.Msg) {
		msg.Ack()
		if err := conn.WriteMessage(websocket.TextMessage, msg.Data()); err != nil {
			proxy.lc.Errorf("Write error: %v", err)
			return
		}
	})
	if err != nil {
		consumeContext.Stop()
		return
	}

	err = publishMessages(conn, js, ctx, proxy.stream+"."+topic+".wsm")
	if err != nil {
		js.Publish(ctx, proxy.stream+"."+topic+".control", []byte("QUIT"))
		proxy.lc.Errorf("Return from publish: %v", err)
		return
	}
}

// NATSProxy represents the NATS message handler
type NATSProxy struct {
	Lc     logger.LoggingClient
	Stream string
	Server string
}

// HandleResponse handles responses received from NATS and sends them back as HTTP responses
func (n *NATSProxy) HandleResponse(msg *nats.Msg) {
	n.Lc.Debug("Handling response")
	var request dto.Request
	var responseBody []byte = nil
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		n.Lc.Errorf("Error decoding message: %v", err)
		return
	}
	// Build new request
	req, err := http.NewRequest(request.Method, request.Url, bytes.NewReader(request.Body))
	if err != nil {
		n.Lc.Errorf("Error creating request: %v", err)
		return
	}
	// Add request headers
	for key, value := range request.Header {
		req.Header[key] = value
	}
	n.Lc.Debugf("Is websocket request: %v", request.IsWs)
	// Handle websocket requests
	if request.IsWs {
		go func() {
			err := n.handleWebsocketServer(req)
			if err != nil {
				n.Lc.Errorf("Error handle websocket requests: %v", err)
				return
			}
		}()
		return
	}

	n.Lc.Debugf("Attempting to request to %s", request.Url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		n.Lc.Errorf("Error sending request: %v", err)
		return
	}
	defer resp.Body.Close()
	n.Lc.Debugf("Got response: %v", resp)

	responseBody, err = io.ReadAll(resp.Body)
	if err != nil {
		n.Lc.Errorf("Error reading response body: %v", err)
		return
	}

	// Publish HTTP response back to NATS
	responseMsg := dto.Response{
		ID:     request.ID,
		Header: resp.Header,
		Body:   responseBody,
		Status: resp.StatusCode,
		Error:  err,
	}
	payload, err := json.Marshal(responseMsg)
	if err != nil {
		n.Lc.Errorf("Error encoding response: %v", err)
		return
	}
	n.Lc.Debugf("HTTP response size: %.2f MB", float64(len(payload))/1024.0/1024.0)

	err = msg.Respond(payload)
	if err != nil {
		n.Lc.Errorf("Error responding to NATS message: %v", err)
		return
	}
}

func (n *NATSProxy) connectToStreamWithRetry(js jetstream.JetStream) (jetstream.Stream, error) {
	ctx := context.Background()

	var err error
	var stream jetstream.Stream
	retries := 5
	for i := 0; i < retries; i++ {
		stream, err = js.Stream(ctx, n.Stream)
		if err == nil {
			return stream, nil
		}
		time.Sleep(time.Second * 1)
	}
	return nil, err
}

// handleWebsocketServer handles WS connection between nats-proxy and a WS server
func (n *NATSProxy) handleWebsocketServer(req *http.Request) error {
	nc, err := nats.Connect(n.Server)
	if err != nil {
		n.Lc.Errorf("Error connecting to NATS server:", err)
		return err
	}
	defer nc.Close()

	topic := req.Header.Get("X-NODE-ID")
	// Connect to WS server in NodeRed
	u := url.URL{Scheme: "ws", Host: req.URL.Host, Path: req.URL.Path}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		n.Lc.Errorf("Error connecting to target WebSocket server %v", err)
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		n.Lc.Errorf("Error creating stream: %v", err)
		return err
	}

	// Obtain stream to publish and read
	stream, err := n.connectToStreamWithRetry(js)
	if err != nil {
		n.Lc.Errorf("Error creating stream: %v", err)
		return err
	}

	// Create node consumer to read stream
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          "node_consumer_" + topic,
		FilterSubject: n.Stream + "." + topic + ".*",
	})
	if err != nil {
		n.Lc.Errorf("Error creating consumer: %v", err)
		return err
	}

	// Consume NATS message - Write to websocket
	iter, _ := consumer.Messages()
	go workerNode(conn, iter, n.Stream, topic)

	// Read from websocket - Publish to NATS messages
	err = publishMessages(conn, js, ctx, n.Stream+".core."+topic)
	if err != nil {
		n.Lc.Errorf("error from publish function %v", err)
		return err
	}
	return nil
}

// SubsInit initialize NATS subscription to a specific subject (Node ID)
func (n *NATSProxy) SubsInit(nc *nats.Conn, nodeId string) {
	// Subscribe to NATS subject
	_, err := nc.Subscribe(nodeId, n.HandleResponse)
	if err != nil {
		n.Lc.Errorf("Unable to subscribe to NATS: %v", err)
		return
	}
	n.Lc.Infof("Subscribed to subject: %s", nodeId)
}

// publishMessages read websocket messages and publishes them to a NATS topic
func publishMessages(conn *websocket.Conn, js jetstream.JetStream, ctx context.Context, topic string) error {
	fmt.Println("Publishing to topic:", topic)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			return err
		}
		_, err = js.Publish(ctx, topic, message)
		if err != nil {
			fmt.Println("error publishing ->", err)
		}
	}
}

// workerNode iterates through NATS messages and sends them to websocket
func workerNode(conn *websocket.Conn, iter jetstream.MessagesContext, strName string, topic string) {
	for {
		msg, _ := iter.Next()
		if string(msg.Data()) == "QUIT" &&
			msg.Subject() == strName+"."+topic+".control" {
			msg.Ack()
			fmt.Println("Got exit signal, bye bye...")
			break
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg.Data()); err != nil {
			fmt.Println("Write error:", err)
			break
		}
		msg.Ack()
	}
	iter.Stop()
	return
}

// isWebSocket checks the request for websocket upgrade
func isWebSocket(r *http.Request) bool {
	contains := func(key, val string) bool {
		vv := strings.Split(r.Header.Get(key), ",")
		for _, v := range vv {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}
	if !contains("Connection", "upgrade") {
		return false
	}
	if !contains("Upgrade", "websocket") {
		return false
	}
	return true
}
