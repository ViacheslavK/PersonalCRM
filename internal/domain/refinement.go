package domain

import "time"

// RefinementSchedule defines the configured weekly refinement trigger.
type RefinementSchedule struct {
	Enabled bool
	Weekday time.Weekday
	Hour    int
	Minute  int
}

// RefinementState holds persisted scheduling state from app_state.
type RefinementState struct {
	LastCompletedAt *time.Time
	SnoozedUntil    *time.Time
	NotifiedCycle   string
}

// RefinementEvaluation describes whether a weekly refinement is currently due and should notify.
type RefinementEvaluation struct {
	Due          bool
	ShouldNotify bool
	CycleKey     string
	TriggeredAt  time.Time
}

// LatestScheduledTrigger returns the most recent scheduled refinement timestamp <= now
// in now's timezone.
func LatestScheduledTrigger(sched RefinementSchedule, now time.Time) time.Time {
	daysBack := (int(now.Weekday()) - int(sched.Weekday) + 7) % 7
	candidateDate := now.AddDate(0, 0, -daysBack)
	trigger := time.Date(
		candidateDate.Year(),
		candidateDate.Month(),
		candidateDate.Day(),
		sched.Hour,
		sched.Minute,
		0,
		0,
		now.Location(),
	)
	if trigger.After(now) {
		trigger = trigger.AddDate(0, 0, -7)
	}
	return trigger
}

// EvaluateRefinement determines whether Weekly Refinement is currently due (including
// catch-up after machine downtime) and whether a desktop notification should be dispatched.
func EvaluateRefinement(sched RefinementSchedule, state RefinementState, now time.Time) RefinementEvaluation {
	if !sched.Enabled {
		return RefinementEvaluation{}
	}

	trigger := LatestScheduledTrigger(sched, now)
	cycleKey := trigger.Format("2006-01-02T15:04")

	// Already completed at or after the most recent scheduled trigger.
	if state.LastCompletedAt != nil && !state.LastCompletedAt.Before(trigger) {
		return RefinementEvaluation{
			Due:          false,
			ShouldNotify: false,
			CycleKey:     cycleKey,
			TriggeredAt:  trigger,
		}
	}

	// Currently snoozed.
	if state.SnoozedUntil != nil && now.Before(*state.SnoozedUntil) {
		return RefinementEvaluation{
			Due:          false,
			ShouldNotify: false,
			CycleKey:     cycleKey,
			TriggeredAt:  trigger,
		}
	}

	shouldNotify := state.NotifiedCycle != cycleKey
	return RefinementEvaluation{
		Due:          true,
		ShouldNotify: shouldNotify,
		CycleKey:     cycleKey,
		TriggeredAt:  trigger,
	}
}

// ShouldRemindTask determines whether an opt-in task deadline reminder should fire at now.
func ShouldRemindTask(t Task, lead time.Duration, now time.Time) bool {
	if !t.ReminderEnabled || !t.IsOpen() || t.DueAt == nil || t.ReminderSentAt != nil {
		return false
	}
	notifyAt := t.DueAt.Add(-lead)
	return !now.Before(notifyAt)
}
