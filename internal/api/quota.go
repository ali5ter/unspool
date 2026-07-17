package api

// DailyQuota is Google's default per-project unit budget (PRD §2.4). It is
// per Google Cloud project and cannot be bought up.
const DailyQuota = 10000

// Quota tracks units spent this process. There is no cross-run persistence
// by design (PRD §6.4) — sync on explicit action only, never a timer.
type Quota struct {
	spent int
}

// NewQuota returns a zeroed Quota.
func NewQuota() *Quota {
	return &Quota{}
}

// Spend records units consumed by an API call.
func (q *Quota) Spend(units int) {
	q.spent += units
}

// Spent returns total units consumed so far this process.
func (q *Quota) Spent() int {
	return q.spent
}

// Remaining returns the estimated units left in today's project-wide budget,
// assuming this process is the only consumer.
func (q *Quota) Remaining() int {
	return DailyQuota - q.spent
}
