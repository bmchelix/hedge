package dto

// Request represents a http request structure
type Request struct {
	ID     string              `json:"id"`
	Header map[string][]string `json:"header"`
	Method string              `json:"method"`
	Url    string              `json:"url"`
	Body   []byte              `json:"body"`
	IsWs   bool                `json:"isWs"`
	Status int                 `json:"status"`
	Error  string              `json:"error"`
}

// Response represents a http response structure
type Response struct {
	ID     string              `json:"id"`
	Header map[string][]string `json:"header"`
	Body   []byte              `json:"body"`
	IsWs   bool                `json:"isWs"`
	Status int                 `json:"status"`
	Error  error               `json:"error"`
}
