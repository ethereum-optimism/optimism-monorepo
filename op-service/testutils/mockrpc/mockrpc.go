package mockrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

var (
	ErrNoMoreCalls     = errors.New("no more calls")
	ErrNoMatchingCalls = errors.New("no matching calls")
)

type jsonRPCReq struct {
	ID     json.RawMessage `json:"id"`
	Params json.RawMessage `json:"params"`
	Method string          `json:"method"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type jsonRPCResp struct {
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

func newResp(id json.RawMessage, result any) jsonRPCResp {
	return jsonRPCResp{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func newErrResp(id json.RawMessage, errCode int, err error) jsonRPCResp {
	return jsonRPCResp{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    errCode,
			Message: err.Error(),
		},
	}
}

type rpcCall struct {
	Method        string          `json:"method"`
	ParamsMatcher ParamsMatcher   `json:"-"`
	Params        json.RawMessage `json:"params"`
	Result        any             `json:"result"`
	Err           string          `json:"err"`
	ErrCode       int             `json:"errCode"`
}

type MockRPC struct {
	calls []rpcCall
	lgr   log.Logger

	lis net.Listener
	err error
}

type Option func(*MockRPC)

func WithExpectationsFile(t *testing.T, path string) Option {
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	var calls []rpcCall
	require.NoError(t, json.NewDecoder(f).Decode(&calls))

	return func(rpc *MockRPC) {
		for _, call := range calls {
			rpc.calls = append(rpc.calls, rpcCall{
				Method:        call.Method,
				ParamsMatcher: JSONParamsMatcher(call.Params),
				Result:        call.Result,
				Err:           call.Err,
				ErrCode:       call.ErrCode,
			})
		}
	}
}

func NewMockRPC(t *testing.T, lgr log.Logger, opts ...Option) *MockRPC {
	m := &MockRPC{
		lgr: lgr,
	}
	for _, opt := range opts {
		opt(m)
	}

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	m.lis = lis

	srv := &http.Server{
		Handler: m,
	}

	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(m.lis)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	timer := time.NewTimer(10 * time.Millisecond)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-timer.C:
	}

	t.Cleanup(func() {
		require.NoError(t, srv.Shutdown(context.Background()))
		require.NoError(t, <-errCh)
	})

	return m
}

func (m *MockRPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.err != nil {
		m.writeResp(w, newErrResp(nil, -32601, m.err))
		return
	}

	if r.Method != http.MethodPost {
		m.lgr.Warn("method not allowed", "method", r.Method)
		http.Error(w, "only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.lgr.Warn("error reading request body", "err", err)
		m.writeResp(w, newErrResp(nil, -32700, err))
		return
	}

	var reqs []jsonRPCReq
	if body[0] == '[' {
		if err := json.Unmarshal(body, &reqs); err != nil {
			m.lgr.Warn("error unmarshalling request body", "err", err)
			m.writeResp(w, newErrResp(nil, -32700, err))
			return
		}
	} else {
		var req jsonRPCReq
		if err := json.Unmarshal(body, &req); err != nil {
			m.lgr.Warn("error unmarshalling request body", "err", err)
			m.writeResp(w, newErrResp(nil, -32700, err))
			return
		}
		reqs = append(reqs, req)
	}

	var resps []jsonRPCResp
	for _, req := range reqs {
		if len(m.calls) == 0 {
			m.err = ErrNoMoreCalls
			resps = append(resps, newErrResp(req.ID, -32601, m.err))
			continue
		}

		call := m.calls[0]
		m.calls = m.calls[1:]

		if call.Method != req.Method {
			m.lgr.Warn("method mismatch", "expected", call.Method, "actual", req.Method)
			m.err = ErrNoMatchingCalls
			resps = append(resps, newErrResp(req.ID, -32601, m.err))
			continue
		}

		if !call.ParamsMatcher(req.Params) {
			m.lgr.Warn("params did not match", "method", req.Method)
			m.err = ErrNoMatchingCalls
			resps = append(resps, newErrResp(req.ID, -32602, m.err))
			continue
		}

		var resp jsonRPCResp
		if call.Err == "" {
			resp = newResp(req.ID, call.Result)
		} else {
			resp = newErrResp(req.ID, call.ErrCode, errors.New(call.Err))
		}
		resps = append(resps, resp)
	}

	var respBytes []byte
	if len(resps) == 1 {
		respBytes, err = json.Marshal(resps[0])
	} else {
		respBytes, err = json.Marshal(resps)
	}
	if err != nil {
		m.lgr.Warn("error marshalling response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		m.lgr.Warn("error writing response", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (m *MockRPC) Endpoint() string {
	return fmt.Sprintf("http://%s", m.lis.Addr().String())
}

func (m *MockRPC) AssertExpectations(t *testing.T) {
	require.NoError(t, m.err)
	require.Empty(t, m.calls)
}

func (m *MockRPC) writeResp(w http.ResponseWriter, in jsonRPCResp) {
	respBytes, err := json.Marshal(in)
	if err != nil {
		m.lgr.Warn("error marshalling response", "err", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		m.lgr.Warn("error writing response", "err", err)
		return
	}
}
