package httputil

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartHTTPServer(t *testing.T) {
	testSetup := func(t *testing.T) (srv *HTTPServer, reqRespBlock chan chan chan struct{}) {
		reqRespBlock = make(chan chan chan struct{}, 10)
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, r.Context().Err())
			respBlock := make(chan chan struct{})
			reqRespBlock <- respBlock
			select {
			case block := <-respBlock:
				block <- struct{}{}
				w.WriteHeader(http.StatusTeapot)
			case <-r.Context().Done():
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		})

		srv, err := StartHTTPServer("localhost:0", h, WithHTTPOptions(WithTimeouts(HTTPTimeouts{
			ReadTimeout:       time.Minute,
			ReadHeaderTimeout: time.Minute,
			WriteTimeout:      time.Minute,
			IdleTimeout:       time.Minute,
		})))
		require.NoError(t, err)
		require.False(t, srv.Closed())
		return srv, reqRespBlock
	}

	t.Run("basics", func(t *testing.T) {
		srv, reqRespBlock := testSetup(t)
		// test basics
		go func() {
			req := <-reqRespBlock // take request
			block := make(chan struct{})
			req <- block // start response
			<-block      // unblock response
		}()
		resp, err := http.Get("http://" + srv.Addr().String() + "/")
		require.NoError(t, err)
		assert.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusTeapot, resp.StatusCode, "I am a teapot")
		assert.NoError(t, srv.Close())
		assert.True(t, srv.Closed())
	})

	t.Run("force-shutdown", func(t *testing.T) {
		srv, reqRespBlock := testSetup(t)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			resp, err := http.Get("http://" + srv.Addr().String() + "/")
			assert.ErrorContains(t, err, "EOF") // error must indicate connection is force-closed
			if resp != nil {
				assert.NoError(t, resp.Body.Close()) // makes linter happy
			}
			wg.Done()
		}()
		req := <-reqRespBlock // take the request
		block := make(chan struct{})
		req <- block // start response
		// just force-shutdown the server
		assert.NoError(t, srv.Close())
		wg.Wait()
		// only now unblock the response
		<-block
		require.True(t, srv.Closed())
	})

	t.Run("graceful", func(t *testing.T) {
		srv, reqRespBlock := testSetup(t)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			resp, err := http.Get("http://" + srv.Addr().String() + "/")
			assert.NoError(t, err)
			assert.NoError(t, resp.Body.Close())
			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "service unavailable when shutting down")
			wg.Done()
		}()
		// Wait for a request, but don't start a response to it, just try to shut down the server
		// The base-context will be shut down, allowing the server to stop waiting for the user,
		// and gracefully tell the user it's not able to continue.
		<-reqRespBlock
		assert.NoError(t, srv.Shutdown(context.Background()))
		wg.Wait()
		require.True(t, srv.Closed())
	})

	t.Run("restart", func(t *testing.T) {
		srv, reqRespBlock := testSetup(t)

		request := func() {
			t.Log("making request")
			// test basics
			go func() {
				req := <-reqRespBlock // take request
				block := make(chan struct{})
				req <- block // start response
				<-block      // unblock response
			}()
			resp, err := http.Get("http://" + srv.Addr().String() + "/")
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusTeapot, resp.StatusCode, "I am a teapot")
		}
		request()
		t.Log("closing 1")
		require.NoError(t, srv.Close())
		require.True(t, srv.Closed())
		t.Log("starting back up")
		require.NoError(t, srv.Start())
		request()
		t.Log("closing 2")
		require.NoError(t, srv.Close())
		require.True(t, srv.Closed())
	})
}
