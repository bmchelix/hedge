package functions

import (
	"hedge/app-services/nats-proxy/dto"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"bytes"
	"context"
	"encoding/json"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type fields struct {
	nc     NATSClient
	lc     logger.LoggingClient
	Lc     logger.LoggingClient
	Stream string
	Server string
}

var flds fields
var u *utils.HedgeMockUtils
var nconn *nats.Conn

func init() {
	conn, js, shutdown, err := NewNATSServer()
	if err != nil {
		panic(err)
	}
	nconn = conn
	defer shutdown()
	ctx := context.Background()
	u = utils.NewApplicationServiceMock(map[string]string{})

	flds = fields{
		nc:     conn,
		lc:     u.AppService.LoggingClient(),
		Lc:     u.AppService.LoggingClient(),
		Stream: "streamTest",
		Server: "localhost:8080",
	}
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "streamTest",
		Subjects: []string{"*"},
		Storage:  jetstream.MemoryStorage, // For speed in tests.
	})
}

func TestHTTPProxy_HandleRequest(t *testing.T) {
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	req, _ := http.NewRequest("GET", "https://google.com", bytes.NewReader([]byte("")))
	req.Header.Set("X-Forwarded-For", "google.com")
	req.Header.Set("X-NODE-ID", "localhost")
	resp := httptest.NewRecorder()
	arg := args{
		w: resp,
		r: req,
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"Request Passed", flds, arg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &HTTPProxy{
				nc: tt.fields.nc,
				lc: tt.fields.lc,
			}
			proxy.HandleRequest(tt.args.w, tt.args.r)
		})
	}
}

func TestNATSProxy_HandleResponse(t *testing.T) {
	type args struct {
		msg *nats.Msg
	}
	request := dto.Request{
		ID:     "localhost",
		Header: nil,
		Method: "GET",
		Url:    "https://google.com",
		Body:   nil,
		IsWs:   false,
		Status: 0,
		Error:  "",
	}
	payload, _ := json.Marshal(request)
	arg := args{msg: &nats.Msg{
		Subject: "",
		Reply:   "",
		Header:  nil,
		Data:    payload,
		Sub:     nil,
	}}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"Response Passed", flds, arg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NATSProxy{
				Lc: tt.fields.Lc,
			}
			n.HandleResponse(tt.args.msg)
		})
	}
}

func TestNATSProxy_SubsInit(t *testing.T) {
	type args struct {
		nc     *nats.Conn
		nodeId string
	}
	arg := args{
		nc:     nconn,
		nodeId: "localhost",
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"Init Passed", flds, arg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NATSProxy{
				Lc: tt.fields.Lc,
			}
			n.SubsInit(tt.args.nc, tt.args.nodeId)
		})
	}
}

func TestNewHTTPProxy(t *testing.T) {
	type args struct {
		nc     NATSClient
		lc     logger.LoggingClient
		stream string
		server string
	}
	arg := args{
		nc:     nconn,
		lc:     u.AppService.LoggingClient(),
		stream: "streamTest",
		server: "localhost:4222",
	}
	htp := NewHTTPProxy(nconn, u.AppService.LoggingClient(), "streamTest", "localhost:4222")
	tests := []struct {
		name string
		args args
		want *HTTPProxy
	}{
		{"New HttpPoxy Passed", arg, htp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewHTTPProxy(tt.args.nc, tt.args.lc, tt.args.stream, tt.args.server); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewHTTPProxy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPProxy_handleWebSocketClient(t *testing.T) {
	type fields struct {
		nc     NATSClient
		lc     logger.LoggingClient
		stream string
		server string
	}
	filds := fields{
		nc:     nconn,
		lc:     u.AppService.LoggingClient(),
		stream: "streamTest",
		server: "localhost:4222",
	}
	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	req, _ := http.NewRequest("GET", "https://google.com", bytes.NewReader([]byte("")))
	req.Header.Set("X-NODE-ID", "localhost")
	resp := httptest.NewRecorder()
	arg := args{
		w: resp,
		r: req,
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"WS Client Passed", filds, arg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := &HTTPProxy{
				nc:     tt.fields.nc,
				lc:     tt.fields.lc,
				stream: tt.fields.stream,
				server: tt.fields.server,
			}
			proxy.handleWebSocketClient(tt.args.w, tt.args.r)
		})
	}
}

func TestNATSProxy_handleWebsocketServer(t *testing.T) {
	type fields struct {
		Lc     logger.LoggingClient
		Stream string
		Server string
	}
	filds := fields{
		Lc:     u.AppService.LoggingClient(),
		Stream: "streamTest",
		Server: "localhost:4222",
	}
	type args struct {
		req *http.Request
	}
	req, _ := http.NewRequest("GET", "https://google.com", bytes.NewReader([]byte("")))
	req.Header.Set("X-NODE-ID", "localhost")
	arg := args{
		req: req,
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"WS Server Passed", filds, arg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NATSProxy{
				Lc:     tt.fields.Lc,
				Stream: tt.fields.Stream,
				Server: tt.fields.Server,
			}
			n.handleWebsocketServer(tt.args.req)
		})
	}
}

//func Test_publishMessages(t *testing.T) {
//	type args struct {
//		conn  *websocket.Conn
//		js    jetstream.JetStream
//		ctx   context.Context
//		topic string
//	}
//	js, _ := jetstream.New(nconn)
//	ctx := context.Background()
//	//conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080", nil)
//	//if err != nil {
//	//}
//	arg := args{
//		conn:  nconn,
//		js:    js,
//		ctx:   ctx,
//		topic: "",
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//	}{
//		{"Publish Passed", arg, false},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := publishMessages(tt.args.conn, tt.args.js, tt.args.ctx, tt.args.topic); (err != nil) != tt.wantErr {
//				t.Errorf("publishMessages() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
//
//func Test_workerNode(t *testing.T) {
//	type args struct {
//		conn    *websocket.Conn
//		iter    jetstream.MessagesContext
//		strName string
//		topic   string
//	}
//	tests := []struct {
//		name string
//		args args
//	}{
//		{},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			workerNode(tt.args.conn, tt.args.iter, tt.args.strName, tt.args.topic)
//		})
//	}
//}
//
//func Test_isWebSocket(t *testing.T) {
//	type args struct {
//		r *http.Request
//	}
//	req, _ := http.NewRequest("GET", "", nil)
//	req2, _ := http.NewRequest("GET", "", nil)
//	req.Header.Set("Connection", "upgrade")
//	req.Header.Set("Upgrade", "websocket")
//	arg := args{r: req}
//	arg2 := args{r: req2}
//	tests := []struct {
//		name string
//		args args
//		want bool
//	}{
//		{"Websocket Passed", arg, true},
//		{"Websocket Failed", arg2, false},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := isWebSocket(tt.args.r); got != tt.want {
//				t.Errorf("isWebSocket() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
