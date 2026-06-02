package serve

import (
	"context"
	"sync"
	"time"
)

// Job tracks the state of a single background /v1/responses request.
//
// Jobs are created with status "in_progress" when a client submits a request
// with "background": true. A goroutine runs the agent, populates Result,
// and transitions Status to "completed" / "failed". Clients poll
// GET /v1/responses/{id} to retrieve the final ResponsesObject.
type Job struct {
	ID         string
	Status     string // "in_progress" | "completed" | "failed" | "cancelled"
	Result     *ResponsesObject
	Cancel     context.CancelFunc
	CreatedAt  time.Time
	FinishedAt time.Time
}

// JobStore is the in-memory job registry for background responses.
//
// Single-process cell serve, so a mutex-protected map is sufficient. Jobs
// survive only while the server runs — clients should not assume cross-restart
// durability.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewJobStore returns an empty JobStore.
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*Job)}
}

// Create inserts a new job in "in_progress" state and returns it.
// cancel is the context.CancelFunc bound to the goroutine running the agent.
func (s *JobStore) Create(id string, cancel context.CancelFunc) *Job {
	j := &Job{
		ID:        id,
		Status:    "in_progress",
		Cancel:    cancel,
		CreatedAt: time.Now(),
	}
	s.mu.Lock()
	s.jobs[id] = j
	s.mu.Unlock()
	return j
}

// Get returns a snapshot of the job by id, or (Job{}, false) if not found.
// The snapshot is a value copy taken under the read lock; callers can read
// its fields without further synchronization. Mutations to the snapshot do
// not affect the stored job.
func (s *JobStore) Get(id string) (Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	return *j, true
}

// Complete transitions the job to a terminal state with the given result.
// status must be "completed" or "failed".
func (s *JobStore) Complete(id string, status string, result *ResponsesObject) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return
	}
	// Don't overwrite a cancelled job — cancel wins.
	if j.Status == "cancelled" {
		return
	}
	j.Status = status
	j.Result = result
	j.FinishedAt = time.Now()
}

// Sweep evicts terminal jobs whose FinishedAt is older than now-ttl.
// In-progress jobs are never evicted regardless of age. Returns the count
// of evicted entries.
//
// Intended for periodic invocation from a goroutine bound to the server
// lifetime; the caller chooses cadence and ttl.
func (s *JobStore) Sweep(now time.Time, ttl time.Duration) int {
	cutoff := now.Add(-ttl)
	evicted := 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, j := range s.jobs {
		if j.Status == "in_progress" {
			continue
		}
		if j.FinishedAt.Before(cutoff) {
			delete(s.jobs, id)
			evicted++
		}
	}
	return evicted
}

// Cancel marks the job as cancelled and triggers its context cancel func.
// Returns (snapshot, true) if the job was found, (Job{}, false) otherwise.
// Idempotent: cancelling an already-cancelled job is a no-op success.
func (s *JobStore) Cancel(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Job{}, false
	}
	if j.Status == "in_progress" {
		j.Status = "cancelled"
		j.FinishedAt = time.Now()
		if j.Cancel != nil {
			j.Cancel()
		}
	}
	return *j, true
}

// setFinishedAt is a test-only helper that backdates a job's FinishedAt so
// Sweep can be exercised without sleeping. Kept on the store (not exported
// loose) so it inherits the same lock discipline as real mutators.
func (s *JobStore) setFinishedAt(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.FinishedAt = t
	}
}
