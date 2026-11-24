package domain

import (
	"fmt"
	"time"
)

// CycleInfo describes whether a Routine is due at a point in time and its unique cycle key.
type CycleInfo struct {
	IsDue    bool
	CycleKey string
}

// ActiveCycle deterministically computes whether a routine is due at now (evaluated in
// now's local calendar timezone) and the cycle key representing the active period.
// Changing cycles automatically resets checklist completion without background cron jobs.
func ActiveCycle(r Routine, now time.Time) (CycleInfo, error) {
	if err := r.Validate(); err != nil {
		return CycleInfo{}, err
	}

	dateStr := now.Format("2006-01-02")

	switch r.ScheduleType {
	case ScheduleDaily:
		return CycleInfo{
			IsDue:    true,
			CycleKey: dateStr,
		}, nil

	case ScheduleWeekly:
		year, week := now.ISOWeek()
		return CycleInfo{
			IsDue:    true,
			CycleKey: fmt.Sprintf("%04d-W%02d", year, week),
		}, nil

	case ScheduleMonthly:
		return CycleInfo{
			IsDue:    true,
			CycleKey: now.Format("2006-01"),
		}, nil

	case ScheduleWeekdays:
		today := int(now.Weekday())
		for _, wd := range r.ScheduleConfig.Weekdays {
			if wd == today {
				return CycleInfo{
					IsDue:    true,
					CycleKey: dateStr,
				}, nil
			}
		}
		return CycleInfo{IsDue: false, CycleKey: ""}, nil

	case ScheduleEveryNDays:
		anchorStr := r.ScheduleConfig.AnchorDate
		if anchorStr == "" {
			if !r.CreatedAt.IsZero() {
				anchorStr = r.CreatedAt.In(now.Location()).Format("2006-01-02")
			} else {
				anchorStr = dateStr
			}
		}
		anchorTime, err := time.ParseInLocation("2006-01-02", anchorStr, now.Location())
		if err != nil {
			return CycleInfo{}, fmt.Errorf("parse anchor_date %q: %w", anchorStr, err)
		}

		days := calendarDaysBetween(anchorTime, now)
		if days < 0 {
			return CycleInfo{IsDue: false, CycleKey: ""}, nil
		}

		cycleIndex := days / r.ScheduleConfig.IntervalDays
		return CycleInfo{
			IsDue:    true,
			CycleKey: fmt.Sprintf("n%d:%s:%d", r.ScheduleConfig.IntervalDays, anchorStr, cycleIndex),
		}, nil

	default:
		return CycleInfo{}, fmt.Errorf("unsupported schedule type %q", r.ScheduleType)
	}
}

// IsCompletedForCycle returns whether the routine is checked off for its current active cycle.
func IsCompletedForCycle(r Routine, now time.Time) (bool, CycleInfo, error) {
	info, err := ActiveCycle(r, now)
	if err != nil {
		return false, CycleInfo{}, err
	}
	if !info.IsDue || info.CycleKey == "" {
		return false, info, nil
	}
	return r.CompletedCycleKey == info.CycleKey, info, nil
}

// ShouldRemindRoutine determines whether an opt-in routine reminder should fire at now.
func ShouldRemindRoutine(r Routine, now time.Time) (bool, string, error) {
	if !r.Active || !r.ReminderEnabled {
		return false, "", nil
	}
	completed, info, err := IsCompletedForCycle(r, now)
	if err != nil {
		return false, "", err
	}
	if !info.IsDue || completed || r.LastRemindedCycle == info.CycleKey {
		return false, info.CycleKey, nil
	}
	if r.ReminderTime != "" {
		parsed, err := time.Parse("15:04", r.ReminderTime)
		if err != nil {
			return false, "", err
		}
		dueAt := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
		if now.Before(dueAt) {
			return false, info.CycleKey, nil
		}
	}
	return true, info.CycleKey, nil
}

// calendarDaysBetween computes whole local calendar days between start and end,
// avoiding DST 23h/25h truncation bugs by normalizing dates to UTC midnight.
func calendarDaysBetween(start, end time.Time) int {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(e.Sub(s).Hours() / 24)
}
