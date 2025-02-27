package altda

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

type KVStore interface {
	// Get retrieves the given key if it's present in the key-value data store.
	Get(ctx context.Context, key []byte) ([]byte, error)
	// Put inserts the given value into the key-value data store.
	Put(ctx context.Context, key []byte, value []byte) error
}

type DAServer struct {
	log            log.Logger
	endpoint       string
	store          KVStore
	tls            *httputil.ServerTLSConfig
	httpServer     *http.Server
	listener       net.Listener
	useGenericComm bool
}

func NewDAServer(host string, port int, store KVStore, log log.Logger, useGenericComm bool) *DAServer {
	endpoint := net.JoinHostPort(host, strconv.Itoa(port))
	return &DAServer{
		log:      log,
		endpoint: endpoint,
		store:    store,
		httpServer: &http.Server{
			Addr: endpoint,
		},
		useGenericComm: useGenericComm,
	}
}

func (d *DAServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/get/", d.HandleGet)
	mux.HandleFunc("/put/", d.HandlePut)
	mux.HandleFunc("/put", d.HandlePut)

	d.httpServer.Handler = mux

	listener, err := net.Listen("tcp", d.endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	d.listener = listener

	d.endpoint = listener.Addr().String()
	errCh := make(chan error, 1)
	go func() {
		if d.tls != nil {
			if err := d.httpServer.ServeTLS(d.listener, "", ""); err != nil {
				errCh <- err
			}
		} else {
			if err := d.httpServer.Serve(d.listener); err != nil {
				errCh <- err
			}
		}
	}()

	// verify that the server comes up
	tick := time.NewTimer(10 * time.Millisecond)
	defer tick.Stop()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	case <-tick.C:
		return nil
	}
}

func (d *DAServer) HandleGet(w http.ResponseWriter, r *http.Request) {
	d.log.Debug("GET", "url", r.URL)

	route := path.Dir(r.URL.Path)
	if route != "/get" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := path.Base(r.URL.Path)
	comm, err := hexutil.Decode(key)
	if err != nil {
		d.log.Error("Failed to decode commitment", "err", err, "key", key)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	input, err := d.store.Get(r.Context(), comm)
	if err != nil && errors.Is(err, ErrNotFound) {
		d.log.Error("Commitment not found", "key", key, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		d.log.Error("Failed to read commitment", "err", err, "key", key)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(input); err != nil {
		d.log.Error("Failed to write pre-image", "err", err, "key", key)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (d *DAServer) HandlePut(w http.ResponseWriter, r *http.Request) {
	d.log.Info("PUT", "url", r.URL)

	route := path.Dir(r.URL.Path)
	if route != "/put" && r.URL.Path != "/put" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	input, err := io.ReadAll(r.Body)
	if err != nil {
		d.log.Error("Failed to read request body", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if r.URL.Path == "/put" || r.URL.Path == "/put/" { // without commitment
		var comm []byte
		if d.useGenericComm {
			n, err := rand.Int(rand.Reader, big.NewInt(99999999999999))
			if err != nil {
				d.log.Error("Failed to generate commitment", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			comm = append(comm, 0x01)
			comm = append(comm, 0xff)
			comm = append(comm, n.Bytes()...)

		} else {
			comm = NewKeccak256Commitment(input).Encode()
		}

		if err = d.store.Put(r.Context(), comm, input); err != nil {
			d.log.Error("Failed to store commitment to the DA server", "err", err, "comm", comm)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		d.log.Info("stored commitment", "key", hex.EncodeToString(comm), "input_len", len(input))

		if _, err := w.Write(comm); err != nil {
			d.log.Error("Failed to write commitment request body", "err", err, "comm", comm)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		key := path.Base(r.URL.Path)
		comm, err := hexutil.Decode(key)
		if err != nil {
			d.log.Error("Failed to decode commitment", "err", err, "key", key)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := d.store.Put(r.Context(), comm, input); err != nil {
			d.log.Error("Failed to store commitment to the DA server", "err", err, "key", key)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (b *DAServer) HttpEndpoint() string {
	return fmt.Sprintf("http://%s", b.listener.Addr().String())
}

func (b *DAServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = b.httpServer.Shutdown(ctx)
	return nil
}
