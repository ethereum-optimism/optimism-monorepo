package altda

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum/go-ethereum/log"
)

func TestFakeDAServer_OutOfOrderResponses(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	daServer := NewFakeDAServer("localhost", 0, logger)
	daServer.SetOutOfOrderResponses(true)

	// Channel to track completion order
	completionOrder := make(chan int, 2)

	// Start two concurrent requests
	var wg sync.WaitGroup
	wg.Add(2)

	// First request
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PUT", "/data", nil)

		daServer.HandlePut(w, r)
		completionOrder <- 1
	}()

	// Small delay to ensure first request starts first
	time.Sleep(100 * time.Millisecond)

	// Second request
	go func() {
		defer wg.Done()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PUT", "/data", nil)

		daServer.HandlePut(w, r)
		completionOrder <- 2
	}()

	// Wait for both requests to complete
	wg.Wait()
	close(completionOrder)

	// Check completion order
	var order []int
	for n := range completionOrder {
		order = append(order, n)
	}

	// Second request should complete before first
	if len(order) != 2 {
		t.Fatalf("expected 2 requests to complete, got %d", len(order))
	}
	if order[0] != 2 || order[1] != 1 {
		t.Errorf("expected completion order [2,1], got %v", order)
	}
}
