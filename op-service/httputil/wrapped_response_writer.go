package httputil

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type WrappedResponseWriter struct {
	StatusCode  int
	ResponseLen int

	w           http.ResponseWriter
	wroteHeader bool

	UpgradeAttempt bool
}

var _ http.Hijacker = (*WrappedResponseWriter)(nil)

func NewWrappedResponseWriter(w http.ResponseWriter) *WrappedResponseWriter {
	return &WrappedResponseWriter{
		StatusCode: 200,
		w:          w,
	}
}

func (w *WrappedResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *WrappedResponseWriter) Write(bytes []byte) (int, error) {
	n, err := w.w.Write(bytes)
	w.ResponseLen += n
	return n, err
}

func (w *WrappedResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}

	w.wroteHeader = true
	w.StatusCode = statusCode
	w.w.WriteHeader(statusCode)
}

// Hijack implements http.Hijacker, so the WrappedResponseWriter is
// compatible as middleware for websocket-upgrades that take over the connection.
func (w *WrappedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.UpgradeAttempt = true
	h, ok := w.w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response-writer is not a http.Hijacker, cannot turn it into raw connection")
	}
	return h.Hijack()
}
