package domain_test

import (
	"testing"
	"time"

	"personalcrm/internal/domain"
)

func TestMonotonicULID(t *testing.T) {
	a := domain.NewID()
	b := domain.NewID()
	if a == "" || b == "" || a == b {
		t.Fatalf("expected distinct non-empty ULIDs, got %q and %q", a, b)
	}
	if !(a < b) {
		t.Fatalf("expected monotonic ascending ULIDs, got %q >= %q", a, b)
	}
}

func TestProjectRefinementEligibility(t *testing.T) {
	cases := []struct {
		status   domain.ProjectStatus
		eligible bool
	}{
		{domain.ProjectActive, true},
		{domain.ProjectWaiting, true},
		{domain.ProjectOnHold, false},
		{domain.ProjectFinished, false},
		{domain.ProjectRejected, false},
	}
	for _, tc := range cases {
		p := domain.Project{Title: "P", Status: tc.status, Priority: domain.PriorityMedium}
		if got := p.IsRefinementEligible(); got != tc.eligible {
			t.Errorf("status %s: expected eligible=%v, got %v", tc.status, tc.eligible, got)
		}
	}
}

func TestRoutineActiveCycles(t *testing.T) {
	loc := time.FixedZone("UTC+1", 3600)
	mon := time.Date(2026, 11, 23, 10, 0, 0, 0, loc) // Monday
	tue := time.Date(2026, 11, 24, 10, 0, 0, 0, loc) // Tuesday
	wed := time.Date(2026, 11, 25, 10, 0, 0, 0, loc) // Wednesday
	sun := time.Date(2026, 11, 29, 23, 0, 0, 0, loc) // Sunday (same ISO week)
	nextMon := time.Date(2026, 11, 30, 8, 0, 0, 0, loc)
	nextMonth := time.Date(2026, 12, 1, 8, 0, 0, 0, loc)

	// Daily
	daily := domain.Routine{Title: "Stretch", ScheduleType: domain.ScheduleDaily, Active: true}
	c1, err := domain.ActiveCycle(daily, mon)
	if err != nil || !c1.IsDue || c1.CycleKey != "2026-11-23" {
		t.Fatalf("daily mon: %+v err=%v", c1, err)
	}
	daily.CompletedCycleKey = c1.CycleKey
	doneMon, _, _ := domain.IsCompletedForCycle(daily, mon)
	doneTue, _, _ := domain.IsCompletedForCycle(daily, tue)
	if !doneMon || doneTue {
		t.Fatalf("daily reset failed: doneMon=%v doneTue=%v", doneMon, doneTue)
	}

	// Weekly (ISO week rollover)
	weekly := domain.Routine{Title: "Clean desk", ScheduleType: domain.ScheduleWeekly, Active: true}
	wMon, _ := domain.ActiveCycle(weekly, mon)
	wSun, _ := domain.ActiveCycle(weekly, sun)
	wNext, _ := domain.ActiveCycle(weekly, nextMon)
	if wMon.CycleKey != wSun.CycleKey || wMon.CycleKey == wNext.CycleKey {
		t.Fatalf("weekly keys wrong: mon=%s sun=%s nextMon=%s", wMon.CycleKey, wSun.CycleKey, wNext.CycleKey)
	}

	// Monthly
	monthly := domain.Routine{Title: "Pay rent", ScheduleType: domain.ScheduleMonthly, Active: true}
	mNov, _ := domain.ActiveCycle(monthly, nextMon)
	mDec, _ := domain.ActiveCycle(monthly, nextMonth)
	if mNov.CycleKey != "2026-11" || mDec.CycleKey != "2026-12" {
		t.Fatalf("monthly keys wrong: nov=%s dec=%s", mNov.CycleKey, mDec.CycleKey)
	}

	// Specific weekdays: Mon(1), Wed(3), Fri(5)
	mwf := domain.Routine{
		Title:        "Workout",
		ScheduleType: domain.ScheduleWeekdays,
		ScheduleConfig: domain.RoutineScheduleConfig{
			Weekdays: []int{int(time.Monday), int(time.Wednesday), int(time.Friday)},
		},
		Active: true,
	}
	if c, err := domain.ActiveCycle(mwf, mon); err != nil || !c.IsDue {
		t.Fatalf("expected Mon due, got %+v err=%v", c, err)
	}
	if c, err := domain.ActiveCycle(mwf, tue); err != nil || c.IsDue {
		t.Fatalf("expected Tue NOT due, got %+v err=%v", c, err)
	}
	if c, err := domain.ActiveCycle(mwf, wed); err != nil || !c.IsDue {
		t.Fatalf("expected Wed due, got %+v err=%v", c, err)
	}

	// Every N days (every 3 days from 2026-11-23)
	every3 := domain.Routine{
		Title:        "Water plants",
		ScheduleType: domain.ScheduleEveryNDays,
		ScheduleConfig: domain.RoutineScheduleConfig{
			IntervalDays: 3,
			AnchorDate:   "2026-11-23",
		},
		Active: true,
	}
	day0, _ := domain.ActiveCycle(every3, mon)
	day2, _ := domain.ActiveCycle(every3, wed)
	day3, _ := domain.ActiveCycle(every3, time.Date(2026, 11, 26, 9, 0, 0, 0, loc))
	beforeAnchor, _ := domain.ActiveCycle(every3, time.Date(2026, 11, 22, 9, 0, 0, 0, loc))

	if beforeAnchor.IsDue {
		t.Fatalf("expected not due before anchor")
	}
	if day0.CycleKey != day2.CycleKey {
		t.Fatalf("day 0 and day 2 should share cycle 0: %s vs %s", day0.CycleKey, day2.CycleKey)
	}
	if day0.CycleKey == day3.CycleKey {
		t.Fatalf("day 3 should roll into cycle 1: both %s", day0.CycleKey)
	}
}

func TestShouldRemindRoutine(t *testing.T) {
	loc := time.UTC
	r := domain.Routine{
		Title:           "Read 20m",
		ScheduleType:    domain.ScheduleDaily,
		Active:          true,
		ReminderEnabled: true,
		ReminderTime:    "20:00",
	}

	beforeTime := time.Date(2026, 11, 24, 19, 30, 0, 0, loc)
	atTime := time.Date(2026, 11, 24, 20, 5, 0, 0, loc)

	ok, _, err := domain.ShouldRemindRoutine(r, beforeTime)
	if err != nil || ok {
		t.Fatalf("should not remind before 20:00, got ok=%v err=%v", ok, err)
	}

	ok, key, err := domain.ShouldRemindRoutine(r, atTime)
	if err != nil || !ok || key != "2026-11-24" {
		t.Fatalf("expected reminder at 20:05, got ok=%v key=%s err=%v", ok, key, err)
	}

	r.LastRemindedCycle = key
	ok, _, _ = domain.ShouldRemindRoutine(r, atTime)
	if ok {
		t.Fatal("should not re-remind in same cycle")
	}
}

func TestWeeklyRefinementEvaluation(t *testing.T) {
	loc := time.UTC
	sched := domain.RefinementSchedule{
		Enabled: true,
		Weekday: time.Sunday,
		Hour:    18,
		Minute:  0,
	}

	// Sunday 17:00 -> latest trigger is previous Sunday 18:00.
	sunBefore := time.Date(2026, 11, 22, 17, 0, 0, 0, loc)
	trig := domain.LatestScheduledTrigger(sched, sunBefore)
	wantPrev := time.Date(2026, 11, 15, 18, 0, 0, 0, loc)
	if !trig.Equal(wantPrev) {
		t.Fatalf("expected %v, got %v", wantPrev, trig)
	}

	// Catch-up: machine boots on Tuesday 09:00 after missing Sunday 18:00.
	tue := time.Date(2026, 11, 24, 9, 0, 0, 0, loc)
	prevDone := time.Date(2026, 11, 16, 10, 0, 0, 0, loc)
	eval := domain.EvaluateRefinement(sched, domain.RefinementState{LastCompletedAt: &prevDone}, tue)
	if !eval.Due || !eval.ShouldNotify {
		t.Fatalf("expected due & shouldNotify on Tuesday catch-up, got %+v", eval)
	}

	// Already notified for this cycle -> Due remains true (Dashboard banner), ShouldNotify becomes false.
	eval2 := domain.EvaluateRefinement(sched, domain.RefinementState{
		LastCompletedAt: &prevDone,
		NotifiedCycle:   eval.CycleKey,
	}, tue)
	if !eval2.Due || eval2.ShouldNotify {
		t.Fatalf("expected banner due without repeat toast, got %+v", eval2)
	}

	// Snoozed until Wednesday -> not due while snoozed, due + notify after expiry.
	snoozeEnd := time.Date(2026, 11, 25, 9, 0, 0, 0, loc)
	evalSnoozed := domain.EvaluateRefinement(sched, domain.RefinementState{
		LastCompletedAt: &prevDone,
		SnoozedUntil:    &snoozeEnd,
	}, tue)
	if evalSnoozed.Due || evalSnoozed.ShouldNotify {
		t.Fatalf("expected suppressed while snoozed, got %+v", evalSnoozed)
	}

	afterSnooze := time.Date(2026, 11, 25, 9, 5, 0, 0, loc)
	evalExpired := domain.EvaluateRefinement(sched, domain.RefinementState{
		LastCompletedAt: &prevDone,
		SnoozedUntil:    &snoozeEnd,
	}, afterSnooze)
	if !evalExpired.Due || !evalExpired.ShouldNotify {
		t.Fatalf("expected due & notify after snooze expired, got %+v", evalExpired)
	}

	// Completed after Sunday 18:00 -> no longer due.
	doneNow := time.Date(2026, 11, 24, 9, 10, 0, 0, loc)
	evalDone := domain.EvaluateRefinement(sched, domain.RefinementState{LastCompletedAt: &doneNow}, doneNow)
	if evalDone.Due || evalDone.ShouldNotify {
		t.Fatalf("expected cleared after completion, got %+v", evalDone)
	}
}

func TestShouldRemindTask(t *testing.T) {
	now := time.Date(2026, 11, 24, 14, 15, 0, 0, time.UTC)
	due := time.Date(2026, 11, 24, 15, 0, 0, 0, time.UTC)

	task := domain.Task{
		Title:           "Submit form",
		Status:          domain.TaskOpen,
		Priority:        domain.PriorityHigh,
		DueAt:           &due,
		ReminderEnabled: true,
	}
	if !domain.ShouldRemindTask(task, time.Hour, now) {
		t.Fatal("expected reminder within 1h lead window")
	}
	if domain.ShouldRemindTask(task, 30*time.Minute, now) {
		t.Fatal("should not remind outside 30m lead window")
	}

	task.Status = domain.TaskDone
	if domain.ShouldRemindTask(task, time.Hour, now) {
		t.Fatal("should not remind done task")
	}
}
