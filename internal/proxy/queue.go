// Package proxy implements the HTTP proxy server.
package proxy

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// QueueConfig holds configuration for the request queue.
type QueueConfig struct {
	Enabled     bool
	MaxSize     int           // Maximum number of requests to queue
	MaxWait     time.Duration // Maximum time a request can wait in queue
	RetryDelay  time.Duration // Delay between retry attempts
	MaxRetries  int           // Maximum number of retries per request
}

// DefaultQueueConfig returns the default queue configuration.
func DefaultQueueConfig() *QueueConfig {
	return &QueueConfig{
		Enabled:    false,
		MaxSize:    100,
		MaxWait:    30 * time.Second,
		RetryDelay: 1 * time.Second,
		MaxRetries: 3,
	}
}

// QueuedRequest represents a request waiting in the queue.
type QueuedRequest struct {
	Body       []byte
	CreatedAt  time.Time
	RetryCount int
	ResultCh   chan QueueResult
}

// QueueResult holds the result of a queued request.
type QueueResult struct {
	Response *http.Response
	Error    error
}

// RequestQueue manages request queuing during provider outages.
type RequestQueue struct {
	config     *QueueConfig
	queue      *list.List
	mu         sync.Mutex
	cond       *sync.Cond
	closed     bool
	processing int32
	paused     int32

	// Metrics
	totalQueued   int64
	totalDequeued int64
	totalDropped  int64
	totalRetried  int64
	totalExpired  int64
}

// NewRequestQueue creates a new request queue.
func NewRequestQueue(config *QueueConfig) *RequestQueue {
	q := &RequestQueue{
		config: config,
		queue:  list.New(),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds a request to the queue.
// Returns an error if the queue is full or closed.
func (q *RequestQueue) Enqueue(body []byte) (chan QueueResult, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, errors.New("queue is closed")
	}

	if q.queue.Len() >= q.config.MaxSize {
		atomic.AddInt64(&q.totalDropped, 1)
		return nil, errors.New("queue is full")
	}

	resultCh := make(chan QueueResult, 1)
	req := &QueuedRequest{
		Body:      body,
		CreatedAt: time.Now(),
		ResultCh:  resultCh,
	}

	q.queue.PushBack(req)
	atomic.AddInt64(&q.totalQueued, 1)

	// Signal that there's work to do
	q.cond.Signal()

	return resultCh, nil
}

// Dequeue removes and returns the next request from the queue.
// Blocks until a request is available or the queue is closed.
func (q *RequestQueue) Dequeue(ctx context.Context) (*QueuedRequest, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for {
		// Check context first
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if q.closed && q.queue.Len() == 0 {
			return nil, errors.New("queue is closed and empty")
		}

		// Wait if paused
		for atomic.LoadInt32(&q.paused) == 1 && !q.closed {
			q.cond.Wait()
		}

		if q.queue.Len() > 0 {
			elem := q.queue.Front()
			q.queue.Remove(elem)
			req := elem.Value.(*QueuedRequest)

			// Check if request has expired
			if time.Since(req.CreatedAt) > q.config.MaxWait {
				atomic.AddInt64(&q.totalExpired, 1)
				req.ResultCh <- QueueResult{Error: errors.New("request timed out in queue")}
				close(req.ResultCh)
				continue // Get next request
			}

			atomic.AddInt64(&q.totalDequeued, 1)
			return req, nil
		}

		// Wait for new items or close
		q.cond.Wait()
	}
}

// Pause temporarily pauses processing (called during outages).
func (q *RequestQueue) Pause() {
	atomic.StoreInt32(&q.paused, 1)
}

// Resume resumes processing after an outage.
func (q *RequestQueue) Resume() {
	atomic.StoreInt32(&q.paused, 0)
	q.mu.Lock()
	q.cond.Broadcast()
	q.mu.Unlock()
}

// IsPaused returns whether the queue is paused.
func (q *RequestQueue) IsPaused() bool {
	return atomic.LoadInt32(&q.paused) == 1
}

// Close closes the queue and signals all waiters.
func (q *RequestQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true

	// Drain remaining requests with error
	for elem := q.queue.Front(); elem != nil; elem = elem.Next() {
		req := elem.Value.(*QueuedRequest)
		req.ResultCh <- QueueResult{Error: errors.New("queue closed")}
		close(req.ResultCh)
	}
	q.queue.Init()

	q.cond.Broadcast()
}

// Len returns the current number of requests in the queue.
func (q *RequestQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.queue.Len()
}

// Stats returns queue statistics.
func (q *RequestQueue) Stats() (queued, dequeued, dropped, retried, expired int64, length int, paused bool) {
	q.mu.Lock()
	length = q.queue.Len()
	q.mu.Unlock()

	queued = atomic.LoadInt64(&q.totalQueued)
	dequeued = atomic.LoadInt64(&q.totalDequeued)
	dropped = atomic.LoadInt64(&q.totalDropped)
	retried = atomic.LoadInt64(&q.totalRetried)
	expired = atomic.LoadInt64(&q.totalExpired)
	paused = q.IsPaused()
	return
}

// IncrementRetried increments the retry counter.
func (q *RequestQueue) IncrementRetried() {
	atomic.AddInt64(&q.totalRetried, 1)
}

// QueueMiddleware creates HTTP middleware that queues requests during outages.
func QueueMiddleware(queue *RequestQueue) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only queue API requests (POST /v1/messages)
			if r.URL.Path != "/v1/messages" || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			// If not paused, proceed normally
			if !queue.IsPaused() {
				next.ServeHTTP(w, r)
				return
			}

			// Queue is paused (outage mode), queue the request
			// Note: The actual queuing logic will be handled by the handler
			// This middleware just adds a header to indicate queue status
			w.Header().Set("X-CLASP-Queue-Status", "paused")
			next.ServeHTTP(w, r)
		})
	}
}

// writeQueueError writes a queue-related error response.
func writeQueueError(w http.ResponseWriter, status int, message string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", string(rune(retryAfter)))
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    "overloaded_error",
			"message": message,
		},
	})
}

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	failureThreshold int
	successThreshold int
	timeout          time.Duration

	failures      int32
	successes     int32
	state         int32 // 0=closed, 1=open, 2=half-open
	lastFailure   time.Time
	mu            sync.RWMutex
}

const (
	circuitClosed   int32 = 0
	circuitOpen     int32 = 1
	circuitHalfOpen int32 = 2
)

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	state := atomic.LoadInt32(&cb.state)

	switch state {
	case circuitClosed:
		return true
	case circuitOpen:
		cb.mu.RLock()
		elapsed := time.Since(cb.lastFailure)
		cb.mu.RUnlock()

		if elapsed > cb.timeout {
			// Transition to half-open
			if atomic.CompareAndSwapInt32(&cb.state, circuitOpen, circuitHalfOpen) {
				atomic.StoreInt32(&cb.successes, 0)
			}
			return true
		}
		return false
	case circuitHalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	state := atomic.LoadInt32(&cb.state)

	if state == circuitHalfOpen {
		successes := atomic.AddInt32(&cb.successes, 1)
		if int(successes) >= cb.successThreshold {
			// Transition to closed
			atomic.StoreInt32(&cb.state, circuitClosed)
			atomic.StoreInt32(&cb.failures, 0)
		}
	} else if state == circuitClosed {
		// Reset failure count on success
		atomic.StoreInt32(&cb.failures, 0)
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	state := atomic.LoadInt32(&cb.state)

	if state == circuitHalfOpen {
		// Transition back to open
		cb.mu.Lock()
		cb.lastFailure = time.Now()
		cb.mu.Unlock()
		atomic.StoreInt32(&cb.state, circuitOpen)
		return
	}

	failures := atomic.AddInt32(&cb.failures, 1)
	if int(failures) >= cb.failureThreshold && state == circuitClosed {
		// Transition to open
		cb.mu.Lock()
		cb.lastFailure = time.Now()
		cb.mu.Unlock()
		atomic.StoreInt32(&cb.state, circuitOpen)
	}
}

// State returns the current state as a string.
func (cb *CircuitBreaker) State() string {
	switch atomic.LoadInt32(&cb.state) {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// IsOpen returns true if the circuit is open.
func (cb *CircuitBreaker) IsOpen() bool {
	return atomic.LoadInt32(&cb.state) == circuitOpen
}
