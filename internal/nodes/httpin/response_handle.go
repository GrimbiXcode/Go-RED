package httpin

import (
    "net/http"
    "sync"
)

// KeyResponseHandle is the message key an "http in" node stores its
// *ResponseHandle under; "http response" (internal/nodes/httpresponse)
// reads it back out to complete the underlying HTTP request. It survives
// every node's shallow cloneMap helper along the way, since the pointer
// value itself is just copied like any other map value.
const KeyResponseHandle = "_httpResponse"

// ResponseHandle correlates an in-flight net/http request with the message
// that carries it through the flow, so a later node (possibly several
// hops and goroutines away) can complete the response.
type ResponseHandle struct {
    w    http.ResponseWriter
    once sync.Once
    done chan struct{}
}

func newResponseHandle(w http.ResponseWriter) *ResponseHandle {
    return &ResponseHandle{w: w, done: make(chan struct{})}
}

// Write sends statusCode/headers/body and unblocks the handler goroutine
// waiting on Done. Only the first call has an effect - Node-RED's own http
// response node is similarly a no-op if the response was already sent.
func (h *ResponseHandle) Write(statusCode int, headers map[string]string, body []byte) {
    h.once.Do(func() {
        for k, v := range headers {
            h.w.Header().Set(k, v)
        }
        h.w.WriteHeader(statusCode)
        _, _ = h.w.Write(body)
        close(h.done)
    })
}

// Done is closed once Write has run (or the request timed out and this
// package wrote a fallback response itself).
func (h *ResponseHandle) Done() <-chan struct{} {
    return h.done
}
