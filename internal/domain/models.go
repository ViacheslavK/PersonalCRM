package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// EntityType identifies an entity kind for polymorphic links, tags, and progress logs.
// It is intentionally an open string so future pluggable modules (e.g. "person") can
// register their own entity types without changing core schema or code.
type EntityType string

const (
	EntityProject  EntityType = "project"
	EntityTask     EntityType = "task"
	EntityIdea     EntityType = "idea"
	EntityHobby    EntityType = "hobby"
	EntityRoutine  EntityType = "routine"
	EntityNote     EntityType = "note"
	EntityResource EntityType = "resource"
)

func (e EntityType) Valid() bool {
	return strings.TrimSpace(string(e)) != ""
}

// Priority is shared by Projects and Tasks.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

// ProjectStatus represents the lifecycle state of a Project.
type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectWaiting  ProjectStatus = "waiting"
	ProjectOnHold   ProjectStatus = "on_hold"
	ProjectFinished ProjectStatus = "finished"
	ProjectRejected ProjectStatus = "rejected"
)

func (s ProjectStatus) Valid() bool {
	switch s {
	case ProjectActive, ProjectWaiting, ProjectOnHold, ProjectFinished, ProjectRejected:
		return true
	default:
		return false
	}
}

// Project represents a scoped effort aimed at achieving a concrete future result.
type Project struct {
	ID          string
	Title       string
	Description string
	Goal        string
	Status      ProjectStatus
	Priority    Priority
	StartDate   *time.Time
	TargetDate  *time.Time
	FinishedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p Project) Validate() error {
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("project title is required")
	}
	if !p.Status.Valid() {
		return fmt.Errorf("invalid project status %q", p.Status)
	}
	if !p.Priority.Valid() {
		return fmt.Errorf("invalid project priority %q", p.Priority)
	}
	return nil
}

// IsRefinementEligible returns true when the project should appear in Weekly Refinement
// (any project not finished, rejected, or on_hold).
func (p Project) IsRefinementEligible() bool {
	return p.Status != ProjectFinished && p.Status != ProjectRejected && p.Status != ProjectOnHold
}

// TaskStatus represents the lifecycle state of a concrete next action.
type TaskStatus string

const (
	TaskOpen       TaskStatus = "open"
	TaskInProgress TaskStatus = "in_progress"
	TaskBlocked    TaskStatus = "blocked"
	TaskDone       TaskStatus = "done"
	TaskCancelled  TaskStatus = "cancelled"
)

func (s TaskStatus) Valid() bool {
	switch s {
	case TaskOpen, TaskInProgress, TaskBlocked, TaskDone, TaskCancelled:
		return true
	default:
		return false
	}
}

// Task represents an actionable step (typically belonging to a Project).
type Task struct {
	ID              string
	ProjectID       string // Empty when standalone
	Title           string
	Description     string
	Status          TaskStatus
	Priority        Priority
	DueAt           *time.Time
	ReminderEnabled bool
	ReminderSentAt  *time.Time
	Position        int
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (t Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("task title is required")
	}
	if !t.Status.Valid() {
		return fmt.Errorf("invalid task status %q", t.Status)
	}
	if !t.Priority.Valid() {
		return fmt.Errorf("invalid task priority %q", t.Priority)
	}
	return nil
}

func (t Task) IsOpen() bool {
	return t.Status != TaskDone && t.Status != TaskCancelled
}

// IdeaStatus represents the maturity state of an Idea in the Ideas Tracker (Inbox).
type IdeaStatus string

const (
	IdeaInbox     IdeaStatus = "inbox"
	IdeaExploring IdeaStatus = "exploring"
	IdeaReady     IdeaStatus = "ready"
	IdeaParked    IdeaStatus = "parked"
	IdeaPromoted  IdeaStatus = "promoted"
	IdeaDiscarded IdeaStatus = "discarded"
)

func (s IdeaStatus) Valid() bool {
	switch s {
	case IdeaInbox, IdeaExploring, IdeaReady, IdeaParked, IdeaPromoted, IdeaDiscarded:
		return true
	default:
		return false
	}
}

// Idea represents an inbox thought or not-yet-started project with research context.
type Idea struct {
	ID                string
	Title             string
	Description       string
	Status            IdeaStatus
	Source            string
	Excitement        *int // Optional 1..5 score
	PromotedProjectID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (i Idea) Validate() error {
	if strings.TrimSpace(i.Title) == "" {
		return errors.New("idea title is required")
	}
	if !i.Status.Valid() {
		return fmt.Errorf("invalid idea status %q", i.Status)
	}
	if i.Excitement != nil && (*i.Excitement < 1 || *i.Excitement > 5) {
		return fmt.Errorf("idea excitement must be between 1 and 5, got %d", *i.Excitement)
	}
	return nil
}

// IsRefinementEligible returns true for ideas that should be triaged during Weekly Refinement.
func (i Idea) IsRefinementEligible() bool {
	return i.Status == IdeaInbox || i.Status == IdeaExploring || i.Status == IdeaReady
}

// PromoteIdeaInput specifies extra details gathered when promoting an Idea into a Project.
type PromoteIdeaInput struct {
	IdeaID       string
	ProjectTitle string // Optional override; defaults to Idea.Title
	Goal         string
	Description  string // Optional override; defaults to Idea.Description
	Priority     Priority
	TargetDate   *time.Time
	InitialTasks []string
	HobbyIDs     []string
}

// HobbyStatus represents the engagement state of a Hobby.
type HobbyStatus string

const (
	HobbyActive   HobbyStatus = "active"
	HobbyOnHold   HobbyStatus = "on_hold"
	HobbyRejected HobbyStatus = "rejected"
)

func (s HobbyStatus) Valid() bool {
	switch s {
	case HobbyActive, HobbyOnHold, HobbyRejected:
		return true
	default:
		return false
	}
}

// Hobby represents an ongoing interest or life domain without a fixed end target.
type Hobby struct {
	ID           string
	Name         string
	Description  string
	CurrentFocus string
	Status       HobbyStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (h Hobby) Validate() error {
	if strings.TrimSpace(h.Name) == "" {
		return errors.New("hobby name is required")
	}
	if !h.Status.Valid() {
		return fmt.Errorf("invalid hobby status %q", h.Status)
	}
	return nil
}

// ScheduleType defines the recurrence pattern of a Routine.
type ScheduleType string

const (
	ScheduleDaily      ScheduleType = "daily"
	ScheduleWeekly     ScheduleType = "weekly"
	ScheduleMonthly    ScheduleType = "monthly"
	ScheduleEveryNDays ScheduleType = "every_n_days"
	ScheduleWeekdays   ScheduleType = "weekdays"
)

func (s ScheduleType) Valid() bool {
	switch s {
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthly, ScheduleEveryNDays, ScheduleWeekdays:
		return true
	default:
		return false
	}
}

// RoutineScheduleConfig holds custom schedule parameters serialized as JSON.
type RoutineScheduleConfig struct {
	IntervalDays int   `json:"interval_days,omitempty"` // For every_n_days (>= 1)
	AnchorDate   string `json:"anchor_date,omitempty"`  // YYYY-MM-DD local anchor for every_n_days
	Weekdays     []int `json:"weekdays,omitempty"`      // 0=Sunday .. 6=Saturday for weekdays
}

// Routine represents a simple recurring checklist item that resets each cycle.
type Routine struct {
	ID                string
	Title             string
	Description       string
	ScheduleType      ScheduleType
	ScheduleConfig    RoutineScheduleConfig
	Active            bool
	ReminderEnabled   bool
	ReminderTime      string // Optional "HH:MM" local time
	CompletedCycleKey string
	LastCompletedAt   *time.Time
	LastRemindedCycle string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (r Routine) Validate() error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("routine title is required")
	}
	if !r.ScheduleType.Valid() {
		return fmt.Errorf("invalid schedule type %q", r.ScheduleType)
	}
	switch r.ScheduleType {
	case ScheduleEveryNDays:
		if r.ScheduleConfig.IntervalDays < 1 {
			return errors.New("every_n_days routine requires interval_days >= 1")
		}
		if r.ScheduleConfig.AnchorDate != "" {
			if _, err := time.Parse("2006-01-02", r.ScheduleConfig.AnchorDate); err != nil {
				return fmt.Errorf("invalid anchor_date %q: %w", r.ScheduleConfig.AnchorDate, err)
			}
		}
	case ScheduleWeekdays:
		if len(r.ScheduleConfig.Weekdays) == 0 {
			return errors.New("weekdays routine requires at least one weekday")
		}
		for _, wd := range r.ScheduleConfig.Weekdays {
			if wd < 0 || wd > 6 {
				return fmt.Errorf("invalid weekday number %d (must be 0..6)", wd)
			}
		}
	}
	if r.ReminderTime != "" {
		if _, err := time.Parse("15:04", r.ReminderTime); err != nil {
			return fmt.Errorf("invalid reminder_time %q: %w", r.ReminderTime, err)
		}
	}
	return nil
}

// Note is a standalone reusable Markdown note linkable to any entity.
type Note struct {
	ID        string
	Title     string
	Content   string
	Source    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (n Note) Validate() error {
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("note title is required")
	}
	return nil
}

// ResourceType classifies external references and links.
type ResourceType string

const (
	ResourceLink   ResourceType = "link"
	ResourceBook   ResourceType = "book"
	ResourceCourse ResourceType = "course"
	ResourceTool   ResourceType = "tool"
	ResourceFile   ResourceType = "file"
	ResourceOther  ResourceType = "other"
)

func (t ResourceType) Valid() bool {
	switch t {
	case ResourceLink, ResourceBook, ResourceCourse, ResourceTool, ResourceFile, ResourceOther:
		return true
	default:
		return false
	}
}

// ResourceStatus tracks consumption progress for a Resource.
type ResourceStatus string

const (
	ResourceToConsume  ResourceStatus = "to_consume"
	ResourceInProgress ResourceStatus = "in_progress"
	ResourceReference  ResourceStatus = "reference"
)

func (s ResourceStatus) Valid() bool {
	switch s {
	case ResourceToConsume, ResourceInProgress, ResourceReference:
		return true
	default:
		return false
	}
}

// Resource is a standalone reusable link, book, course, tool, or file reference.
type Resource struct {
	ID           string
	Title        string
	URLOrPath    string
	ResourceType ResourceType
	Status       ResourceStatus
	Notes        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (r Resource) Validate() error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("resource title is required")
	}
	if strings.TrimSpace(r.URLOrPath) == "" {
		return errors.New("resource url_or_path is required")
	}
	if !r.ResourceType.Valid() {
		return fmt.Errorf("invalid resource type %q", r.ResourceType)
	}
	if !r.Status.Valid() {
		return fmt.Errorf("invalid resource status %q", r.Status)
	}
	return nil
}

// EntityLink represents a polymorphic many-to-many relation between any two entities.
type EntityLink struct {
	SourceType   EntityType
	SourceID     string
	TargetType   EntityType
	TargetID     string
	RelationType string
	CreatedAt    time.Time
}

func (l EntityLink) Validate() error {
	if !l.SourceType.Valid() || strings.TrimSpace(l.SourceID) == "" {
		return errors.New("link source_type and source_id are required")
	}
	if !l.TargetType.Valid() || strings.TrimSpace(l.TargetID) == "" {
		return errors.New("link target_type and target_id are required")
	}
	if l.SourceType == l.TargetType && l.SourceID == l.TargetID {
		return errors.New("cannot link an entity to itself")
	}
	return nil
}

// ProgressLog records a dated progress or reflection entry on a Project, Hobby, or plugin entity.
type ProgressLog struct {
	ID         string
	EntityType EntityType
	EntityID   string
	LoggedAt   time.Time
	Content    string
	CreatedAt  time.Time
}

func (p ProgressLog) Validate() error {
	if !p.EntityType.Valid() || strings.TrimSpace(p.EntityID) == "" {
		return errors.New("progress log entity_type and entity_id are required")
	}
	if strings.TrimSpace(p.Content) == "" {
		return errors.New("progress log content is required")
	}
	return nil
}

// Tag is a lightweight label attachable polymorphically to any entity.
type Tag struct {
	ID   string
	Name string
}

// RefinementRun records a Weekly Refinement session.
type RefinementRun struct {
	ID               string
	CycleKey         string
	StartedAt        time.Time
	CompletedAt      *time.Time
	ProjectsReviewed int
	IdeasReviewed    int
	RoutinesReviewed int
	Notes            string
}
