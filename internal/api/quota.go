package api

import "sync"

// DailyQuota is Google's default per-project unit budget. It is per Google
// Cloud project and cannot be bought up.
const DailyQuota = 10000

// Quota tracks units spent this process. There is no cross-run persistence
// by design — sync on explicit action only, never a timer.
//
// Guarded by a mutex: channel syncs run concurrently (see
// channelSyncConcurrency in internal/feed), and every one of them spends
// quota through the same Client's Quota — a plain int here would race.
type Quota struct {
	mu    sync.Mutex
	spent int
}

// NewQuota returns a zeroed Quota.
func NewQuota() *Quota {
	return &Quota{}
}

// Spend records units consumed by an API call.
func (q *Quota) Spend(units int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.spent += units
}

// Spent returns total units consumed so far this process.
func (q *Quota) Spent() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.spent
}

// Remaining returns the estimated units left in today's project-wide budget,
// assuming this process is the only consumer.
func (q *Quota) Remaining() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return DailyQuota - q.spent
}
