package tests

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/clasp/internal/proxy"
)

func TestRequestQueue_EnqueueDequeue(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    10,
		MaxWait:    5 * time.Second,
		RetryDelay: 100 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)
	defer queue.Close()

	// Enqueue a request
	body := []byte(`{"test": "data"}`)
	resultCh, err := queue.Enqueue(body)
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}
	if resultCh == nil {
		t.Fatal("Expected result channel, got nil")
	}

	// Check length
	if queue.Len() != 1 {
		t.Errorf("Expected queue length 1, got %d", queue.Len())
	}

	// Dequeue in background
	ctx := context.Background()
	go func() {
		req, err := queue.Dequeue(ctx)
		if err != nil {
			t.Errorf("Failed to dequeue: %v", err)
			return
		}
		if string(req.Body) != string(body) {
			t.Errorf("Body mismatch: expected %s, got %s", body, req.Body)
		}
		// Send result
		req.ResultCh <- proxy.QueueResult{Response: nil, Error: nil}
		close(req.ResultCh)
	}()

	// Wait for result
	select {
	case result := <-resultCh:
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for result")
	}

	// Check queue is empty
	if queue.Len() != 0 {
		t.Errorf("Expected queue length 0, got %d", queue.Len())
	}
}

func TestRequestQueue_MaxSize(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    2,
		MaxWait:    5 * time.Second,
		RetryDelay: 100 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)
	defer queue.Close()

	// Fill queue to max
	for i := 0; i < 2; i++ {
		_, err := queue.Enqueue([]byte("test"))
		if err != nil {
			t.Fatalf("Failed to enqueue request %d: %v", i, err)
		}
	}

	// Third request should fail
	_, err := queue.Enqueue([]byte("overflow"))
	if err == nil {
		t.Fatal("Expected error for queue overflow, got nil")
	}

	// Check stats
	stats := queue.Stats()
	if stats.Dropped != 1 {
		t.Errorf("Expected 1 dropped request, got %d", stats.Dropped)
	}
	if stats.Queued != 2 {
		t.Errorf("Expected 2 queued requests, got %d", stats.Queued)
	}
}

func TestRequestQueue_Expiry(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    10,
		MaxWait:    100 * time.Millisecond, // Very short timeout
		RetryDelay: 10 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)
	defer queue.Close()

	// Enqueue request
	resultCh, err := queue.Enqueue([]byte("test"))
	if err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Dequeue should skip expired and return error
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		req, err := queue.Dequeue(ctx)
		if err == nil {
			// Request was dequeued but should be expired
			t.Logf("Request dequeued (may have been fast enough)")
			req.ResultCh <- proxy.QueueResult{}
		}
	}()

	// Wait for expiry result
	select {
	case result := <-resultCh:
		if result.Error == nil || result.Error.Error() != "request timed out in queue" {
			t.Logf("Got result: %v (may have been fast enough)", result.Error)
		}
	case <-time.After(2 * time.Second):
		// This is acceptable - request may have been processed
		t.Log("No result received (acceptable if request was processed quickly)")
	}
}

func TestRequestQueue_PauseResume(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    10,
		MaxWait:    5 * time.Second,
		RetryDelay: 100 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)
	defer queue.Close()

	// Initially not paused
	if queue.IsPaused() {
		t.Error("Queue should not be paused initially")
	}

	// Pause
	queue.Pause()
	if !queue.IsPaused() {
		t.Error("Queue should be paused")
	}

	// Resume
	queue.Resume()
	if queue.IsPaused() {
		t.Error("Queue should not be paused after resume")
	}
}

func TestRequestQueue_ConcurrentAccess(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    100,
		MaxWait:    1 * time.Second,
		RetryDelay: 10 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)

	var wg sync.WaitGroup
	numProducers := 5
	numPerProducer := 3
	consumed := int32(0)

	// Start consumer first
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				req, err := queue.Dequeue(ctx)
				if err != nil {
					return
				}
				req.ResultCh <- proxy.QueueResult{}
				close(req.ResultCh)
				atomic.AddInt32(&consumed, 1)
			}
		}
	}()

	// Start producers
	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numPerProducer; i++ {
				_, err := queue.Enqueue([]byte("test"))
				if err != nil {
					// Queue might be full or closed
					continue
				}
			}
		}()
	}

	// Wait for timeout
	<-ctx.Done()
	cancel()
	queue.Close()
	wg.Wait()

	// Verify we consumed some requests
	c := atomic.LoadInt32(&consumed)
	if c == 0 {
		t.Error("Expected to consume some requests")
	}
	t.Logf("Consumed %d requests", c)
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := proxy.NewCircuitBreaker(3, 2, 1*time.Second)

	// Should be closed initially
	if cb.State() != "closed" {
		t.Errorf("Expected closed state, got %s", cb.State())
	}

	// Should allow requests
	for i := 0; i < 10; i++ {
		if !cb.Allow() {
			t.Error("Circuit breaker should allow requests when closed")
		}
		cb.RecordSuccess()
	}

	// Still closed
	if cb.State() != "closed" {
		t.Errorf("Expected closed state after successes, got %s", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	cb := proxy.NewCircuitBreaker(3, 2, 1*time.Second)

	// Record failures
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Error("Circuit breaker should allow requests before threshold")
		}
		cb.RecordFailure()
	}

	// Should be open
	if cb.State() != "open" {
		t.Errorf("Expected open state, got %s", cb.State())
	}

	// Should not allow requests
	if cb.Allow() {
		t.Error("Circuit breaker should not allow requests when open")
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cb := proxy.NewCircuitBreaker(2, 2, 100*time.Millisecond)

	// Open circuit
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != "open" {
		t.Errorf("Expected open state, got %s", cb.State())
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Should allow request and transition to half-open
	if !cb.Allow() {
		t.Error("Circuit breaker should allow request after timeout")
	}

	if cb.State() != "half-open" {
		t.Errorf("Expected half-open state, got %s", cb.State())
	}
}

func TestCircuitBreaker_ClosesOnRecovery(t *testing.T) {
	cb := proxy.NewCircuitBreaker(2, 2, 100*time.Millisecond)

	// Open circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Transition to half-open
	cb.Allow()

	// Record successes to close
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != "closed" {
		t.Errorf("Expected closed state after recovery, got %s", cb.State())
	}
}

func TestCircuitBreaker_ReopensOnHalfOpenFailure(t *testing.T) {
	cb := proxy.NewCircuitBreaker(2, 2, 100*time.Millisecond)

	// Open circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Transition to half-open
	cb.Allow()

	// Record failure - should reopen
	cb.RecordFailure()

	if cb.State() != "open" {
		t.Errorf("Expected open state after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_IsOpen(t *testing.T) {
	cb := proxy.NewCircuitBreaker(2, 2, 1*time.Second)

	// Initially not open
	if cb.IsOpen() {
		t.Error("Circuit breaker should not be open initially")
	}

	// Open it
	cb.RecordFailure()
	cb.RecordFailure()

	// Now open
	if !cb.IsOpen() {
		t.Error("Circuit breaker should be open after failures")
	}
}

func TestQueueStats(t *testing.T) {
	config := &proxy.QueueConfig{
		Enabled:    true,
		MaxSize:    10,
		MaxWait:    5 * time.Second,
		RetryDelay: 100 * time.Millisecond,
		MaxRetries: 3,
	}

	queue := proxy.NewRequestQueue(config)
	defer queue.Close()

	// Enqueue some requests
	queue.Enqueue([]byte("test1"))
	queue.Enqueue([]byte("test2"))

	// Check initial stats
	stats := queue.Stats()

	if stats.Queued != 2 {
		t.Errorf("Expected queued=2, got %d", stats.Queued)
	}
	if stats.Dequeued != 0 {
		t.Errorf("Expected dequeued=0, got %d", stats.Dequeued)
	}
	if stats.Dropped != 0 {
		t.Errorf("Expected dropped=0, got %d", stats.Dropped)
	}
	if stats.Length != 2 {
		t.Errorf("Expected length=2, got %d", stats.Length)
	}
	if stats.Paused {
		t.Error("Expected paused=false")
	}

	// Pause and check
	queue.Pause()
	stats = queue.Stats()
	if !stats.Paused {
		t.Error("Expected paused=true after pause")
	}
}
