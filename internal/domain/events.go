package domain

import "time"

// EventType identifies domain lifecycle events published to the pluggable EventBus.
type EventType string

const (
	EventProjectCreated       EventType = "project.created"
	EventProjectUpdated       EventType = "project.updated"
	EventProjectStatusChanged EventType = "project.status_changed"
	EventProjectDeleted       EventType = "project.deleted"

	EventTaskCreated   EventType = "task.created"
	EventTaskCompleted EventType = "task.completed"
	EventTaskDeleted   EventType = "task.deleted"

	EventIdeaCreated  EventType = "idea.created"
	EventIdeaPromoted EventType = "idea.promoted"
	EventIdeaDeleted  EventType = "idea.deleted"

	EventHobbyCreated EventType = "hobby.created"
	EventHobbyUpdated EventType = "hobby.updated"

	EventRoutineCreated   EventType = "routine.created"
	EventRoutineCompleted EventType = "routine.completed"

	EventNoteCreated     EventType = "note.created"
	EventResourceCreated EventType = "resource.created"
	EventProgressLogged  EventType = "progress.logged"

	EventRefinementCompleted EventType = "refinement.completed"
)

// Event carries domain state changes to subscribed extensions and background jobs.
type Event struct {
	Type       EventType
	EntityType EntityType
	EntityID   string
	OccurredAt time.Time
	Payload    map[string]any
}
