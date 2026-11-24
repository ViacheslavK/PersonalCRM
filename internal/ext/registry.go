package ext

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"personalcrm/internal/config"
	"personalcrm/internal/domain"
	"personalcrm/internal/storage"
)

// ModuleInfo summarizes a registered module and its active state.
type ModuleInfo struct {
	ID             string
	Name           string
	Description    string
	Enabled        bool
	DefaultEnabled bool
}

// Registry manages discovery, configuration toggling, migrations, and lifecycle
// of all pluggable PersonalCRM modules.
type Registry struct {
	modules map[string]Module
	order   []string
	enabled map[string]bool
	inited  bool
}

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
		enabled: make(map[string]bool),
	}
}

// Register adds a module to the registry before InitAll is called.
func (r *Registry) Register(m Module) error {
	if m == nil {
		return fmt.Errorf("cannot register nil module")
	}
	id := strings.TrimSpace(m.ID())
	if id == "" {
		return fmt.Errorf("module ID must not be empty")
	}
	if _, exists := r.modules[id]; exists {
		return fmt.Errorf("module %q already registered", id)
	}
	r.modules[id] = m
	r.order = append(r.order, id)
	return nil
}

// Has reports whether a module ID is registered.
func (r *Registry) Has(id string) bool {
	_, ok := r.modules[id]
	return ok
}

// IsEnabled resolves whether a module is enabled according to config (falling back to DefaultEnabled).
func (r *Registry) IsEnabled(id string, cfg config.Config) bool {
	m, ok := r.modules[id]
	if !ok {
		return false
	}
	if val, configured := cfg.Plugins[id]; configured {
		return val
	}
	return m.DefaultEnabled()
}

// InitAll runs namespaced migrations ("ext:<id>") and Init for every enabled module.
func (r *Registry) InitAll(ctx context.Context, store *storage.Store, bus *EventBus, cfg *config.Config) error {
	mctx := ModuleContext{
		Store:  store,
		Bus:    bus,
		Config: cfg,
	}

	for _, id := range r.order {
		m := r.modules[id]
		on := r.IsEnabled(id, *cfg)
		r.enabled[id] = on
		if !on {
			continue
		}

		if migs := m.Migrations(); len(migs) > 0 {
			if err := store.ApplyMigrations(ctx, "ext:"+id, migs); err != nil {
				return fmt.Errorf("module %q migrations: %w", id, err)
			}
		}
		if err := m.Init(ctx, mctx); err != nil {
			return fmt.Errorf("init module %q: %w", id, err)
		}
	}

	r.inited = true
	return nil
}

// List returns metadata for all registered modules sorted by ID.
func (r *Registry) List(cfg config.Config) []ModuleInfo {
	ids := append([]string(nil), r.order...)
	sort.Strings(ids)
	out := make([]ModuleInfo, 0, len(ids))
	for _, id := range ids {
		m := r.modules[id]
		out = append(out, ModuleInfo{
			ID:             id,
			Name:           m.Name(),
			Description:    m.Description(),
			Enabled:        r.IsEnabled(id, cfg),
			DefaultEnabled: m.DefaultEnabled(),
		})
	}
	return out
}

// EnabledModules returns all currently enabled modules in registration order.
func (r *Registry) EnabledModules() []Module {
	var out []Module
	for _, id := range r.order {
		if r.enabled[id] {
			out = append(out, r.modules[id])
		}
	}
	return out
}

// CLICommands aggregates CLI commands from all enabled modules.
func (r *Registry) CLICommands() []CLICommand {
	var out []CLICommand
	for _, m := range r.EnabledModules() {
		out = append(out, m.CLICommands()...)
	}
	return out
}

// TUIViewSpecs aggregates TUI view providers from all enabled modules.
func (r *Registry) TUIViewSpecs() []TUIViewSpec {
	var out []TUIViewSpec
	for _, m := range r.EnabledModules() {
		out = append(out, m.TUIViewSpecs()...)
	}
	return out
}

// RefinementStepSpecs aggregates optional Weekly Refinement steps from enabled modules.
func (r *Registry) RefinementStepSpecs() []RefinementStepSpec {
	var out []RefinementStepSpec
	for _, m := range r.EnabledModules() {
		out = append(out, m.RefinementStepSpecs()...)
	}
	return out
}

// ScheduledJobs aggregates daemon jobs from enabled modules.
func (r *Registry) ScheduledJobs() []ScheduledJob {
	var out []ScheduledJob
	for _, m := range r.EnabledModules() {
		out = append(out, m.ScheduledJobs()...)
	}
	return out
}

// EntityBadges collects optional badges (e.g. "@claude") from enabled modules for an entity.
func (r *Registry) EntityBadges(ctx context.Context, entityType domain.EntityType, entityID string) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, m := range r.EnabledModules() {
		out = append(out, m.EntityBadges(ctx, entityType, entityID)...)
	}
	return out
}

// EntityDetailSections collects optional detail sections from enabled modules for an entity.
func (r *Registry) EntityDetailSections(ctx context.Context, entityType domain.EntityType, entityID string) []DetailSection {
	if r == nil {
		return nil
	}
	var out []DetailSection
	for _, m := range r.EnabledModules() {
		out = append(out, m.EntityDetailSections(ctx, entityType, entityID)...)
	}
	return out
}
