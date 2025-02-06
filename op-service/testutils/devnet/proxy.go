package devnet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum/go-ethereum/log"
)

type jsonRPCReq struct {
	Method string `json:"method"`
}

var copyHeaders = []string{
	"Content-Type",
}

type RetryProxy struct {
	lgr        log.Logger
	upstream   string
	client     *http.Client
	strategy   retry.Strategy
	maxRetries int
	srv        *http.Server
	listenPort int
}

type Option func(*RetryProxy)

func NewRetryProxy(lgr log.Logger, upstream string, opts ...Option) *RetryProxy {
	strategy := &retry.ExponentialStrategy{
		Min:       250 * time.Millisecond,
		Max:       5 * time.Second,
		MaxJitter: 250 * time.Millisecond,
	}

	prox := &RetryProxy{
		lgr:        lgr.New("module", "retryproxy"),
		upstream:   upstream,
		client:     &http.Client{},
		strategy:   strategy,
		maxRetries: 5,
	}

	for _, opt := range opts {
		opt(prox)
	}

	return prox
}

func (p *RetryProxy) Start() error {
	errC := make(chan error, 1)

	go func() {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			errC <- fmt.Errorf("failed to listen: %w", err)
		}

		p.listenPort = ln.Addr().(*net.TCPAddr).Port

		p.srv = &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: p,
		}
		errC <- p.srv.Serve(ln)
	}()

	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case err := <-errC:
		return fmt.Errorf("failed to start server: %w", err)
	case <-timer.C:
		p.lgr.Info("server started", "port", p.listenPort)
		return nil
	}
}

func (p *RetryProxy) Stop() error {
	if p.srv == nil {
		return nil
	}

	return p.srv.Shutdown(context.Background())
}

func (p *RetryProxy) Endpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", p.listenPort)
}

func (p *RetryProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		p.lgr.Error("failed to read request body", "err", err)
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}

	//nolint:bodyClose
	res, resBody, err := retry.Do2(r.Context(), p.maxRetries, p.strategy, func() (*http.Response, []byte, error) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := p.doProxyReq(ctx, reqBody)
		if err != nil {
			p.lgr.Warn("failed to proxy request", "err", err)
			return nil, nil, err
		}

		defer res.Body.Close()
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			p.lgr.Warn("failed to read response body", "err", err)
			return nil, nil, err
		}

		return res, resBody, nil
	})
	if err != nil {
		p.lgr.Error("permanently failed to proxy request", "err", err)
		http.Error(w, "failed to proxy request", http.StatusBadGateway)
		return
	}

	for _, h := range copyHeaders {
		w.Header().Set(h, res.Header.Get(h))
	}

	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, bytes.NewReader(resBody)); err != nil {
		p.lgr.Error("failed to copy response", "err", err)
		http.Error(w, "failed to copy response", http.StatusInternalServerError)
		return
	}

	var jReq jsonRPCReq
	if err := json.Unmarshal(reqBody, &jReq); err != nil {
		p.lgr.Warn("failed to unmarshal request", "err", err)
		return
	}

	p.lgr.Trace("proxied request", "method", jReq.Method, "dur", time.Since(start))
}

func (p *RetryProxy) doProxyReq(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.upstream, bytes.NewReader(body))
	if err != nil {
		panic(fmt.Errorf("failed to create request: %w", err))
	}
	res, err := p.client.Do(req)
	if err != nil {
		p.lgr.Warn("failed to proxy request", "err", err)
		return nil, err
	}
	status := res.StatusCode
	if status != 200 {
		p.lgr.Warn("unexpected status code", "status", status)
		return nil, fmt.Errorf("unexpected status code: %d", status)
	}
	return res, nil
}
