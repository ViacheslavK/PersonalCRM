package ext

import (
	"context"
	"io"
	"time"

	"personalcrm/internal/config"
	"personalcrm/internal/domain"
	"personalcrm/internal/storage"
)

// ModuleContext provides core services to an initialized pluggable module.
type ModuleContext struct {
	Store  *storage.Store
	Bus    *EventBus
	Config *config.Config
}

// CLICommand allows a module to register custom top-level CLI subcommands
// (e.g. "people", "report").
type CLICommand struct {
	Name        string
	Summary     string
	Usage       string
	Run         func(ctx context.Context, args []string, stdout, stderr io.Writer) error
}

// TUIViewSpec advertises an optional TUI tab or panel provided by a module.
// Keeping this metadata framework-agnostic avoids coupling internal/ext to Bubble Tea.
type TUIViewSpec struct {
	ID       string
	Title    string
	Shortcut string
	Factory  func() any // Returns a Bubble Tea model in Phase 4
}

// RefinementStepSpec advertises an optional step appended to Weekly Refinement.
type RefinementStepSpec struct {
	ID      string
	Title   string
	Summary string
	Factory func() any
}

// ScheduledJob defines an optional periodic background task executed by the daemon.
type ScheduledJob struct {
	ID       string
	Interval time.Duration
	Run      func(ctx context.Context, now time.Time) error
}

// DetailSection is a titled block of lines injected by an enabled plugin into a core
// entity's TUI detail panel (e.g. Customer/Contacts on a Project when ext:people is enabled).
type DetailSection struct {
	Title string
	Lines []string
}

// Module is the contract implemented by every optional PersonalCRM plugin/extension.
type Module interface {
	ID() string
	Name() string
	Description() string
	DefaultEnabled() bool

	Migrations() []storage.Migration
	Init(ctx context.Context, mctx ModuleContext) error

	CLICommands() []CLICommand
	TUIViewSpecs() []TUIViewSpec
	RefinementStepSpecs() []RefinementStepSpec
	ScheduledJobs() []ScheduledJob

	// EntityBadges returns optional compact badges (e.g. "@claude") rendered beside an entity in TUI lists.
	EntityBadges(ctx context.Context, entityType domain.EntityType, entityID string) []string
	// EntityDetailSections returns optional extra sections rendered in an entity's TUI detail panel.
	EntityDetailSections(ctx context.Context, entityType domain.EntityType, entityID string) []DetailSection
}

// BaseModule provides no-op defaults so modules only implement hooks they need.
type BaseModule struct{}

func (BaseModule) DefaultEnabled() bool                      { return false }
func (BaseModule) Migrations() []storage.Migration           { return nil }
func (BaseModule) Init(context.Context, ModuleContext) error { return nil }
func (BaseModule) CLICommands() []CLICommand                 { return nil }
func (BaseModule) TUIViewSpecs() []TUIViewSpec               { return nil }
func (BaseModule) RefinementStepSpecs() []RefinementStepSpec { return nil }
func (BaseModule) ScheduledJobs() []ScheduledJob             { return nil }
func (BaseModule) EntityBadges(context.Context, domain.EntityType, string) []string {
	return nil
}
func (BaseModule) EntityDetailSections(context.Context, domain.EntityType, string) []DetailSection {
	return nil
}
