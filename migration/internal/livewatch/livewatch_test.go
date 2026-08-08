package livewatch

import (
	"testing"
	"time"
)

func int64p(v int64) *int64 { return &v }

func baseTime(offsetMinutes int) time.Time {
	return time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC).Add(time.Duration(offsetMinutes) * time.Minute)
}

func TestFirstPollHasNoResolvedOrOngoingChanges(t *testing.T) {
	tr := NewTracker(2)
	poll := Poll{
		Sequence: 1,
		PolledAt: baseTime(0),
		Tables: []TableResult{
			{SourceSchema: "dbo", SourceTable: "SaleLedger", TargetSchema: "public", TargetTable: "business_documents", SourceCount: int64p(100), TargetCount: int64p(100), Status: "matched"},
			{SourceSchema: "dbo", SourceTable: "Purledger", TargetSchema: "public", TargetTable: "business_documents", SourceCount: int64p(50), TargetCount: int64p(48), Status: "mismatched"},
		},
		Bookkeeping: BookkeepingResult{Status: "clear"},
	}

	diff := tr.Observe(poll)

	if diff.PreviousPolledAt != nil {
		t.Fatalf("first poll should have no previous poll time, got %v", diff.PreviousPolledAt)
	}
	if len(diff.NewTableDiscrepancies) != 1 {
		t.Fatalf("expected 1 new table discrepancy, got %d", len(diff.NewTableDiscrepancies))
	}
	if len(diff.OngoingTableDiscrepancies) != 0 || len(diff.ResolvedTableDiscrepancies) != 0 {
		t.Fatalf("first poll must not report ongoing/resolved changes: %+v", diff)
	}
	change := diff.NewTableDiscrepancies[0]
	if change.Confirmed {
		t.Fatal("a discrepancy seen on only 1 poll must not be confirmed with confirmAfter=2")
	}
	if change.ConsecutivePolls != 1 || change.FirstSeenSequence != 1 {
		t.Fatalf("unexpected streak bookkeeping: %+v", change)
	}
	if diff.Status != "pending" {
		t.Fatalf("status = %q, want pending (unconfirmed discrepancy present)", diff.Status)
	}
}

func TestDiscrepancyConfirmedOnSecondConsecutivePoll(t *testing.T) {
	tr := NewTracker(2)
	tr.Observe(Poll{
		Sequence: 1,
		PolledAt: baseTime(0),
		Tables: []TableResult{
			{SourceSchema: "dbo", SourceTable: "Purledger", SourceCount: int64p(50), TargetCount: int64p(48), Status: "mismatched"},
		},
		Bookkeeping: BookkeepingResult{Status: "clear"},
	})

	diff := tr.Observe(Poll{
		Sequence: 2,
		PolledAt: baseTime(15),
		Tables: []TableResult{
			{SourceSchema: "dbo", SourceTable: "Purledger", SourceCount: int64p(51), TargetCount: int64p(48), Status: "mismatched"},
		},
		Bookkeeping: BookkeepingResult{Status: "clear"},
	})

	if len(diff.NewTableDiscrepancies) != 0 {
		t.Fatalf("a discrepancy seen last poll too must be 'ongoing', not 'new': %+v", diff.NewTableDiscrepancies)
	}
	if len(diff.OngoingTableDiscrepancies) != 1 {
		t.Fatalf("expected 1 ongoing table discrepancy, got %d", len(diff.OngoingTableDiscrepancies))
	}
	change := diff.OngoingTableDiscrepancies[0]
	if !change.Confirmed {
		t.Fatal("a discrepancy seen on 2 consecutive polls must be confirmed with confirmAfter=2")
	}
	if change.ConsecutivePolls != 2 || change.FirstSeenSequence != 1 {
		t.Fatalf("unexpected streak bookkeeping: %+v", change)
	}
	if *change.PreviousSourceCount != 50 || *change.CurrentSourceCount != 51 {
		t.Fatalf("expected source count to move from 50 to 51: %+v", change)
	}
	if change.CountGapDelta == nil || *change.CountGapDelta != 1 {
		t.Fatalf("expected the source/target gap to have widened by 1, got %v", change.CountGapDelta)
	}
	if diff.Status != "discrepancy" {
		t.Fatalf("status = %q, want discrepancy once confirmed", diff.Status)
	}
	if diff.PreviousPolledAt == nil || !diff.PreviousPolledAt.Equal(baseTime(0)) {
		t.Fatalf("expected previous polled-at to be the first poll's time, got %v", diff.PreviousPolledAt)
	}
}

func TestTimingSkewResolvesWithoutEverBeingConfirmed(t *testing.T) {
	// Simulates the exact scenario the package doc calls out: a transaction
	// entered seconds before poll 1 fires shows up on the source but not yet
	// on the target; by poll 2 (15 minutes later) the operator has entered it
	// on both sides and it reconciles. It must never be "confirmed".
	tr := NewTracker(2)
	tr.Observe(Poll{
		Sequence: 1,
		PolledAt: baseTime(0),
		Tables: []TableResult{
			{SourceSchema: "dbo", SourceTable: "SaleLedger", SourceCount: int64p(1001), TargetCount: int64p(1000), Status: "mismatched"},
		},
		Bookkeeping: BookkeepingResult{Status: "clear"},
	})

	diff := tr.Observe(Poll{
		Sequence: 2,
		PolledAt: baseTime(15),
		Tables: []TableResult{
			{SourceSchema: "dbo", SourceTable: "SaleLedger", SourceCount: int64p(1001), TargetCount: int64p(1001), Status: "matched"},
		},
		Bookkeeping: BookkeepingResult{Status: "clear"},
	})

	if len(diff.ResolvedTableDiscrepancies) != 1 {
		t.Fatalf("expected the timing skew to resolve, got %+v", diff)
	}
	if diff.ResolvedTableDiscrepancies[0].ConsecutivePolls != 1 {
		t.Fatalf("resolved discrepancy should report it was only seen once: %+v", diff.ResolvedTableDiscrepancies[0])
	}
	if len(diff.NewTableDiscrepancies) != 0 || len(diff.OngoingTableDiscrepancies) != 0 {
		t.Fatalf("no discrepancy should remain open after resolution: %+v", diff)
	}
	if diff.Status != "match" {
		t.Fatalf("status = %q, want match once everything resolves", diff.Status)
	}
}

func TestResolvedDiscrepancyStreakResetsIfItReappearsLater(t *testing.T) {
	tr := NewTracker(2)
	tr.Observe(Poll{Sequence: 1, PolledAt: baseTime(0), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "SRLedger", SourceCount: int64p(10), TargetCount: int64p(9), Status: "mismatched"},
	}})
	tr.Observe(Poll{Sequence: 2, PolledAt: baseTime(15), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "SRLedger", SourceCount: int64p(10), TargetCount: int64p(10), Status: "matched"},
	}})
	diff := tr.Observe(Poll{Sequence: 3, PolledAt: baseTime(30), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "SRLedger", SourceCount: int64p(11), TargetCount: int64p(10), Status: "mismatched"},
	}})

	if len(diff.NewTableDiscrepancies) != 1 {
		t.Fatalf("a discrepancy reappearing after resolution should be 'new' again, got %+v", diff)
	}
	if diff.NewTableDiscrepancies[0].Confirmed {
		t.Fatal("a freshly reappeared discrepancy must not already be confirmed")
	}
	if diff.NewTableDiscrepancies[0].FirstSeenSequence != 3 {
		t.Fatalf("first-seen sequence should reset to the reappearance poll, got %d", diff.NewTableDiscrepancies[0].FirstSeenSequence)
	}
}

func TestMetricDiscrepancyTrackedSeparatelyFromTables(t *testing.T) {
	tr := NewTracker(1) // confirm immediately for this test
	diff := tr.Observe(Poll{
		Sequence: 1,
		PolledAt: baseTime(0),
		Metrics: []MetricResult{
			{Name: "sale_line_total_value_sum", SourceValue: "100.00", TargetValue: "90.00", Difference: "10.00000000", Status: "mismatched"},
			{Name: "purchase_line_total_value_sum", SourceValue: "50.00", TargetValue: "50.00", Difference: "0.00000000", Status: "matched"},
		},
	})

	if len(diff.NewMetricDiscrepancies) != 1 {
		t.Fatalf("expected 1 new metric discrepancy, got %d", len(diff.NewMetricDiscrepancies))
	}
	if diff.NewMetricDiscrepancies[0].Name != "sale_line_total_value_sum" {
		t.Fatalf("unexpected metric flagged: %+v", diff.NewMetricDiscrepancies[0])
	}
	if !diff.NewMetricDiscrepancies[0].Confirmed {
		t.Fatal("confirmAfter=1 should confirm on the first poll")
	}
	if diff.Status != "discrepancy" {
		t.Fatalf("status = %q, want discrepancy", diff.Status)
	}
}

func TestBookkeepingChangeIsNotSubjectToConfirmation(t *testing.T) {
	// Bookkeeping reads target-only state (no cross-database timing race), so
	// unlike table/metric counts, a regression is reported as a discrepancy
	// immediately rather than waiting for a second poll.
	tr := NewTracker(2)
	tr.Observe(Poll{Sequence: 1, PolledAt: baseTime(0), Bookkeeping: BookkeepingResult{Status: "clear"}})
	diff := tr.Observe(Poll{Sequence: 2, PolledAt: baseTime(15), Bookkeeping: BookkeepingResult{
		Status: "review_required", OpenMigrationExceptions: 1, OpenMigrationExceptionRows: 3,
	}})

	if !diff.BookkeepingChanged {
		t.Fatal("expected bookkeeping change to be detected")
	}
	if diff.Status != "discrepancy" {
		t.Fatalf("status = %q, want discrepancy on bookkeeping regression", diff.Status)
	}
	if diff.PreviousBookkeeping == nil || diff.PreviousBookkeeping.Status != "clear" {
		t.Fatalf("expected previous bookkeeping to be recorded: %+v", diff.PreviousBookkeeping)
	}
}

func TestUnchangedBookkeepingIsNotFlagged(t *testing.T) {
	tr := NewTracker(2)
	tr.Observe(Poll{Sequence: 1, PolledAt: baseTime(0), Bookkeeping: BookkeepingResult{Status: "clear"}})
	diff := tr.Observe(Poll{Sequence: 2, PolledAt: baseTime(15), Bookkeeping: BookkeepingResult{Status: "clear"}})

	if diff.BookkeepingChanged {
		t.Fatal("bookkeeping did not change and should not be flagged")
	}
	if diff.Status != "match" {
		t.Fatalf("status = %q, want match", diff.Status)
	}
}

func TestTableDroppedFromScopeDoesNotLeakStreakState(t *testing.T) {
	tr := NewTracker(2)
	tr.Observe(Poll{Sequence: 1, PolledAt: baseTime(0), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "Purledger", SourceCount: int64p(5), TargetCount: int64p(4), Status: "mismatched"},
	}})
	// Next poll's scope no longer includes Purledger (e.g. operator narrowed
	// -config); it should not linger as tracked state forever.
	tr.Observe(Poll{Sequence: 2, PolledAt: baseTime(15), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "SaleLedger", SourceCount: int64p(1), TargetCount: int64p(1), Status: "matched"},
	}})
	// If Purledger comes back discrepant again, it must start a fresh streak,
	// not resume the old one (which would wrongly confirm immediately).
	diff := tr.Observe(Poll{Sequence: 3, PolledAt: baseTime(30), Tables: []TableResult{
		{SourceSchema: "dbo", SourceTable: "Purledger", SourceCount: int64p(6), TargetCount: int64p(4), Status: "mismatched"},
	}})

	if len(diff.NewTableDiscrepancies) != 1 || diff.NewTableDiscrepancies[0].Confirmed {
		t.Fatalf("expected a fresh, unconfirmed discrepancy after the table dropped out of scope: %+v", diff)
	}
}

func TestHasConfirmedDiscrepancyIgnoresPendingOnly(t *testing.T) {
	d := Diff{
		NewTableDiscrepancies: []TableChange{{Confirmed: false}},
	}
	if d.HasConfirmedDiscrepancy() {
		t.Fatal("a pending-only diff must not report a confirmed discrepancy")
	}
	d.NewTableDiscrepancies[0].Confirmed = true
	if !d.HasConfirmedDiscrepancy() {
		t.Fatal("a confirmed change must be detected")
	}
}

func TestNewTrackerDefaultsNonPositiveConfirmAfter(t *testing.T) {
	tr := NewTracker(0)
	if tr.confirmAfter != DefaultConfirmAfter {
		t.Fatalf("confirmAfter = %d, want default %d", tr.confirmAfter, DefaultConfirmAfter)
	}
	tr = NewTracker(-3)
	if tr.confirmAfter != DefaultConfirmAfter {
		t.Fatalf("confirmAfter = %d, want default %d", tr.confirmAfter, DefaultConfirmAfter)
	}
}

func TestCountGapDeltaNilWhenEitherPollLacksCounts(t *testing.T) {
	if got := countGapDelta(nil, int64p(1), int64p(2), int64p(2)); got != nil {
		t.Fatalf("expected nil delta when previous source count missing, got %v", *got)
	}
	if got := countGapDelta(int64p(1), int64p(1), nil, int64p(2)); got != nil {
		t.Fatalf("expected nil delta when current source count missing, got %v", *got)
	}
}
