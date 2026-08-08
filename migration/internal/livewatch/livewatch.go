// Package livewatch implements the poll-over-poll diffing behind
// migration/cmd/livecompare: given two consecutive reconcile.Report polls
// taken minutes apart against a live source, it highlights what changed —
// new discrepancies, discrepancies that resolved themselves, and
// discrepancies that are still open — rather than reprinting the full
// (potentially 700+ table) report every interval.
//
// # Why "confirm on the next poll" instead of a time-window exclusion
//
// A parallel trading day compares a live, actively-changing SQL Server
// source against a live PostgreSQL target while an operator enters the same
// transaction into both systems by hand. A transaction entered a few
// seconds before a poll fires can legitimately exist on one side and not
// the other yet — that is expected timing skew, not a real discrepancy, and
// should not be flagged as one.
//
// The declarative mapping config (migration/maps/*.json) does not carry a
// per-table "last modified" column, so there is no generic, config-driven
// way to exclude "the last N minutes of rows" from a COUNT(*) — some source
// tables have no reliable timestamp column at all. Instead, Tracker treats
// any newly-observed discrepancy as "pending" for its first poll and only
// escalates it to "confirmed" once it has been observed on ConfirmAfter
// consecutive polls (default 2). Since polls run minutes apart (the -interval
// flag, default 15m) and legacy/target entry of the same transaction is a
// matter of seconds to low minutes apart in practice, a same-transaction
// timing race resolves itself well within one interval and is reported as
// "resolved" on the very next poll — it never reaches "confirmed" and never
// demands operator attention. A genuine divergence (a transaction entered on
// only one side, or entered with different totals) persists and gets
// confirmed on the following poll.
package livewatch

import "time"

// DefaultConfirmAfter is how many consecutive polls a table or metric must
// stay discrepant before Tracker reports it as "confirmed" rather than
// "pending". See the package doc for why this substitutes for a per-table
// recency window.
const DefaultConfirmAfter = 2

// TableResult and MetricResult mirror reconcile.TableResult / MetricResult
// structurally (this package intentionally does not import
// migration/internal/reconcile, so its diff logic can be unit tested with
// plain fixtures and stays reusable if the report shape ever needs to
// diverge from the batch reconciler's).
type TableResult struct {
	SourceSchema string
	SourceTable  string
	TargetSchema string
	TargetTable  string
	SourceCount  *int64
	TargetCount  *int64
	Status       string
	Error        string
}

// MetricResult mirrors reconcile.MetricResult.
type MetricResult struct {
	Name        string
	SourceValue string
	TargetValue string
	Difference  string
	Tolerance   float64
	Status      string
	Error       string
}

// BookkeepingResult mirrors reconcile.BookkeepingResult.
type BookkeepingResult struct {
	OpenMigrationExceptions    int64
	OpenMigrationExceptionRows int64
	OpenMigrationAmbiguities   int64
	Status                     string
}

// Poll is one interval's reconciliation snapshot, decorated with the poll
// metadata the tracker needs to sequence and window polls.
type Poll struct {
	Sequence    int
	PolledAt    time.Time
	WindowStart time.Time
	WindowEnd   time.Time
	Tables      []TableResult
	Metrics     []MetricResult
	Bookkeeping BookkeepingResult
}

func matched(status string) bool {
	return status == "matched"
}

// TableChange describes one table whose match status changed, is changing,
// or remains changed between two consecutive polls.
type TableChange struct {
	SourceSchema        string
	SourceTable         string
	TargetSchema        string
	TargetTable         string
	PreviousStatus      string
	CurrentStatus       string
	PreviousSourceCount *int64
	CurrentSourceCount  *int64
	PreviousTargetCount *int64
	CurrentTargetCount  *int64
	CountGapDelta       *int64 // change in (source-target) gap since the previous poll, when both polls have counts
	FirstSeenSequence   int
	ConsecutivePolls    int
	Confirmed           bool
	Error               string
}

// MetricChange describes one metric whose match status changed, is
// changing, or remains changed between two consecutive polls.
type MetricChange struct {
	Name                string
	PreviousStatus      string
	CurrentStatus       string
	PreviousSourceValue string
	CurrentSourceValue  string
	PreviousTargetValue string
	CurrentTargetValue  string
	PreviousDifference  string
	CurrentDifference   string
	FirstSeenSequence   int
	ConsecutivePolls    int
	Confirmed           bool
	Error               string
}

// Diff is what changed between the previous poll and the current one.
type Diff struct {
	Sequence         int
	PolledAt         time.Time
	PreviousPolledAt *time.Time
	WindowStart      time.Time
	WindowEnd        time.Time

	// Status summarizes the poll: "match" (nothing discrepant), "pending"
	// (a discrepancy appeared this poll but has not been confirmed yet), or
	// "discrepancy" (something has stayed discrepant across ConfirmAfter
	// consecutive polls and needs operator attention).
	Status string

	NewTableDiscrepancies      []TableChange
	OngoingTableDiscrepancies  []TableChange
	ResolvedTableDiscrepancies []TableChange

	NewMetricDiscrepancies      []MetricChange
	OngoingMetricDiscrepancies  []MetricChange
	ResolvedMetricDiscrepancies []MetricChange

	BookkeepingChanged  bool
	PreviousBookkeeping *BookkeepingResult
	CurrentBookkeeping  BookkeepingResult
}

// HasConfirmedDiscrepancy reports whether this poll surfaced any
// confirmed (non-pending) table or metric discrepancy, or a bookkeeping
// regression.
func (d Diff) HasConfirmedDiscrepancy() bool {
	for _, c := range d.NewTableDiscrepancies {
		if c.Confirmed {
			return true
		}
	}
	for _, c := range d.OngoingTableDiscrepancies {
		if c.Confirmed {
			return true
		}
	}
	for _, c := range d.NewMetricDiscrepancies {
		if c.Confirmed {
			return true
		}
	}
	for _, c := range d.OngoingMetricDiscrepancies {
		if c.Confirmed {
			return true
		}
	}
	if d.BookkeepingChanged && d.CurrentBookkeeping.Status != "clear" {
		return true
	}
	return false
}

func countGapDelta(prevSource, prevTarget, currSource, currTarget *int64) *int64 {
	if prevSource == nil || prevTarget == nil || currSource == nil || currTarget == nil {
		return nil
	}
	prevGap := *prevSource - *prevTarget
	currGap := *currSource - *currTarget
	delta := currGap - prevGap
	return &delta
}

type tableKey struct {
	sourceSchema, sourceTable string
}

type metricKey struct {
	name string
}

// Tracker holds cross-poll state (discrepancy streaks) and turns each new
// Poll into a Diff against whatever poll preceded it.
type Tracker struct {
	confirmAfter int

	haveLastPoll    bool
	lastPolledAt    time.Time
	lastTables      map[tableKey]TableResult
	lastMetrics     map[metricKey]MetricResult
	lastBookkeeping BookkeepingResult

	tableStreaks   map[tableKey]int
	tableFirstSeq  map[tableKey]int
	metricStreaks  map[metricKey]int
	metricFirstSeq map[metricKey]int
}

// NewTracker builds a Tracker that confirms a discrepancy after it has been
// observed on confirmAfter consecutive polls. A value <= 0 falls back to
// DefaultConfirmAfter.
func NewTracker(confirmAfter int) *Tracker {
	if confirmAfter <= 0 {
		confirmAfter = DefaultConfirmAfter
	}
	return &Tracker{
		confirmAfter:   confirmAfter,
		lastTables:     map[tableKey]TableResult{},
		lastMetrics:    map[metricKey]MetricResult{},
		tableStreaks:   map[tableKey]int{},
		tableFirstSeq:  map[tableKey]int{},
		metricStreaks:  map[metricKey]int{},
		metricFirstSeq: map[metricKey]int{},
	}
}

// Observe folds one poll into the tracker's state and returns what changed
// relative to the immediately preceding poll (if any).
func (t *Tracker) Observe(poll Poll) Diff {
	diff := Diff{
		Sequence:    poll.Sequence,
		PolledAt:    poll.PolledAt,
		WindowStart: poll.WindowStart,
		WindowEnd:   poll.WindowEnd,
		Status:      "match",
	}
	if t.haveLastPoll {
		previous := t.lastPolledAt
		diff.PreviousPolledAt = &previous
	}

	currentTables := make(map[tableKey]TableResult, len(poll.Tables))
	for _, r := range poll.Tables {
		key := tableKey{r.SourceSchema, r.SourceTable}
		currentTables[key] = r
		prev, hadPrev := t.lastTables[key]

		if matched(r.Status) {
			if streak := t.tableStreaks[key]; streak > 0 {
				diff.ResolvedTableDiscrepancies = append(diff.ResolvedTableDiscrepancies, TableChange{
					SourceSchema:        r.SourceSchema,
					SourceTable:         r.SourceTable,
					TargetSchema:        r.TargetSchema,
					TargetTable:         r.TargetTable,
					PreviousStatus:      prev.Status,
					CurrentStatus:       r.Status,
					PreviousSourceCount: prev.SourceCount,
					CurrentSourceCount:  r.SourceCount,
					PreviousTargetCount: prev.TargetCount,
					CurrentTargetCount:  r.TargetCount,
					CountGapDelta:       countGapDelta(prev.SourceCount, prev.TargetCount, r.SourceCount, r.TargetCount),
					FirstSeenSequence:   t.tableFirstSeq[key],
					ConsecutivePolls:    streak,
				})
			}
			delete(t.tableStreaks, key)
			delete(t.tableFirstSeq, key)
			continue
		}

		t.tableStreaks[key]++
		streak := t.tableStreaks[key]
		if streak == 1 {
			t.tableFirstSeq[key] = poll.Sequence
		}
		change := TableChange{
			SourceSchema:       r.SourceSchema,
			SourceTable:        r.SourceTable,
			TargetSchema:       r.TargetSchema,
			TargetTable:        r.TargetTable,
			CurrentStatus:      r.Status,
			CurrentSourceCount: r.SourceCount,
			CurrentTargetCount: r.TargetCount,
			FirstSeenSequence:  t.tableFirstSeq[key],
			ConsecutivePolls:   streak,
			Confirmed:          streak >= t.confirmAfter,
			Error:              r.Error,
		}
		if hadPrev {
			change.PreviousStatus = prev.Status
			change.PreviousSourceCount = prev.SourceCount
			change.PreviousTargetCount = prev.TargetCount
			change.CountGapDelta = countGapDelta(prev.SourceCount, prev.TargetCount, r.SourceCount, r.TargetCount)
		}
		if !hadPrev || matched(prev.Status) {
			diff.NewTableDiscrepancies = append(diff.NewTableDiscrepancies, change)
		} else {
			diff.OngoingTableDiscrepancies = append(diff.OngoingTableDiscrepancies, change)
		}
	}
	// A table absent from this poll but tracked as discrepant before (e.g. a
	// narrower -config was swapped in mid-run) is treated as resolved rather
	// than left dangling, so streak state never leaks past the run.
	for key := range t.tableStreaks {
		if _, present := currentTables[key]; !present {
			delete(t.tableStreaks, key)
			delete(t.tableFirstSeq, key)
		}
	}
	t.lastTables = currentTables

	currentMetrics := make(map[metricKey]MetricResult, len(poll.Metrics))
	for _, m := range poll.Metrics {
		key := metricKey{m.Name}
		currentMetrics[key] = m
		prev, hadPrev := t.lastMetrics[key]

		if matched(m.Status) {
			if streak := t.metricStreaks[key]; streak > 0 {
				diff.ResolvedMetricDiscrepancies = append(diff.ResolvedMetricDiscrepancies, MetricChange{
					Name:                m.Name,
					PreviousStatus:      prev.Status,
					CurrentStatus:       m.Status,
					PreviousSourceValue: prev.SourceValue,
					CurrentSourceValue:  m.SourceValue,
					PreviousTargetValue: prev.TargetValue,
					CurrentTargetValue:  m.TargetValue,
					PreviousDifference:  prev.Difference,
					CurrentDifference:   m.Difference,
					FirstSeenSequence:   t.metricFirstSeq[key],
					ConsecutivePolls:    streak,
				})
			}
			delete(t.metricStreaks, key)
			delete(t.metricFirstSeq, key)
			continue
		}

		t.metricStreaks[key]++
		streak := t.metricStreaks[key]
		if streak == 1 {
			t.metricFirstSeq[key] = poll.Sequence
		}
		change := MetricChange{
			Name:               m.Name,
			CurrentStatus:      m.Status,
			CurrentSourceValue: m.SourceValue,
			CurrentTargetValue: m.TargetValue,
			CurrentDifference:  m.Difference,
			FirstSeenSequence:  t.metricFirstSeq[key],
			ConsecutivePolls:   streak,
			Confirmed:          streak >= t.confirmAfter,
			Error:              m.Error,
		}
		if hadPrev {
			change.PreviousStatus = prev.Status
			change.PreviousSourceValue = prev.SourceValue
			change.PreviousTargetValue = prev.TargetValue
			change.PreviousDifference = prev.Difference
		}
		if !hadPrev || matched(prev.Status) {
			diff.NewMetricDiscrepancies = append(diff.NewMetricDiscrepancies, change)
		} else {
			diff.OngoingMetricDiscrepancies = append(diff.OngoingMetricDiscrepancies, change)
		}
	}
	for key := range t.metricStreaks {
		if _, present := currentMetrics[key]; !present {
			delete(t.metricStreaks, key)
			delete(t.metricFirstSeq, key)
		}
	}
	t.lastMetrics = currentMetrics

	if t.haveLastPoll {
		prevBK := t.lastBookkeeping
		diff.PreviousBookkeeping = &prevBK
		diff.BookkeepingChanged = prevBK.Status != poll.Bookkeeping.Status ||
			prevBK.OpenMigrationExceptions != poll.Bookkeeping.OpenMigrationExceptions ||
			prevBK.OpenMigrationExceptionRows != poll.Bookkeeping.OpenMigrationExceptionRows ||
			prevBK.OpenMigrationAmbiguities != poll.Bookkeeping.OpenMigrationAmbiguities
	}
	diff.CurrentBookkeeping = poll.Bookkeeping
	t.lastBookkeeping = poll.Bookkeeping
	t.lastPolledAt = poll.PolledAt
	t.haveLastPoll = true

	switch {
	case diff.HasConfirmedDiscrepancy():
		diff.Status = "discrepancy"
	case len(diff.NewTableDiscrepancies) > 0 || len(diff.OngoingTableDiscrepancies) > 0 ||
		len(diff.NewMetricDiscrepancies) > 0 || len(diff.OngoingMetricDiscrepancies) > 0:
		diff.Status = "pending"
	default:
		diff.Status = "match"
	}
	return diff
}
