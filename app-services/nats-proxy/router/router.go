package router

import (
	"hedge/app-services/nats-proxy/functions"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"net/http"
)

type Router struct {
	service interfaces.ApplicationService
}

func NewRouter(service interfaces.ApplicationService) *Router {
	router := new(Router)
	router.service = service
	return router
}

func (r Router) LoadRoute(proxy *functions.HTTPProxy) {
	port := ":48200"
	mux := http.NewServeMux()
	mux.HandleFunc("/", proxy.HandleRequest)
	fmt.Println("HTTP-NATS-Proxy listening on port", port)
	err := http.ListenAndServe(port, mux)
	if err != nil {
		fmt.Println("Listen&Serve returned an error", err.Error())
		return
	}
}
