package ext_test

import (
	"context"
	"io"
	"testing"
	"time"

	"personalcrm/internal/config"
	"personalcrm/internal/domain"
	"personalcrm/internal/ext"
	"personalcrm/internal/storage"
)

type sampleModule struct {
	ext.BaseModule
	id         string
	inited     bool
	eventsSeen int
}

func (m *sampleModule) ID() string          { return m.id }
func (m *sampleModule) Name() string        { return "Sample Module" }
func (m *sampleModule) Description() string { return "Test plugin" }
func (m *sampleModule) Migrations() []storage.Migration {
	return []storage.Migration{
		{
			Version: 1,
			Name:    "create_sample",
			SQL:     `CREATE TABLE ext_sample (id TEXT PRIMARY KEY, label TEXT NOT NULL);`,
		},
	}
}
func (m *sampleModule) Init(_ context.Context, mctx ext.ModuleContext) error {
	m.inited = true
	mctx.Bus.Subscribe(domain.EventIdeaCreated, func(_ context.Context, _ domain.Event) error {
		m.eventsSeen++
		return nil
	})
	return nil
}
func (m *sampleModule) CLICommands() []ext.CLICommand {
	return []ext.CLICommand{
		{
			Name:    "sample",
			Summary: "Run sample plugin command",
			Run: func(_ context.Context, _ []string, stdout, _ io.Writer) error {
				_, err := io.WriteString(stdout, "sample-ok\n")
				return err
			},
		},
	}
}

func TestEventBusPubSubAndWildcard(t *testing.T) {
	bus := ext.NewEventBus()
	var specific, all int

	bus.Subscribe(domain.EventProjectCreated, func(_ context.Context, _ domain.Event) error {
		specific++
		return nil
	})
	bus.SubscribeAll(func(_ context.Context, _ domain.Event) error {
		all++
		return nil
	})

	errs := bus.Publish(context.Background(), domain.Event{
		Type:       domain.EventProjectCreated,
		EntityType: domain.EntityProject,
		EntityID:   "01TEST",
		OccurredAt: time.Now().UTC(),
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected publish errors: %v", errs)
	}
	bus.Publish(context.Background(), domain.Event{Type: domain.EventIdeaCreated})

	if specific != 1 || all != 2 {
		t.Fatalf("expected specific=1 all=2, got specific=%d all=%d", specific, all)
	}
}

func TestRegistryConfigToggleMigrationsAndHooks(t *testing.T) {
	ctx := context.Background()
	store, err := storage.OpenInMemory(ctx)
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer store.Close()

	mod := &sampleModule{id: "sample"}
	reg := ext.NewRegistry()
	if err := reg.Register(mod); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Disabled by default -> Init and migrations must NOT run.
	cfg := config.Default()
	bus := ext.NewEventBus()
	if err := reg.InitAll(ctx, store, bus, &cfg); err != nil {
		t.Fatalf("InitAll disabled: %v", err)
	}
	if mod.inited || len(reg.CLICommands()) != 0 {
		t.Fatalf("expected disabled module not initialized")
	}

	// Enable via config -> migrations run, Init subscribes to EventBus, CLI commands exposed.
	cfg.Plugins["sample"] = true
	if err := reg.InitAll(ctx, store, bus, &cfg); err != nil {
		t.Fatalf("InitAll enabled: %v", err)
	}
	if !mod.inited || len(reg.CLICommands()) != 1 {
		t.Fatalf("expected enabled module initialized with 1 CLI command")
	}

	// Verify namespaced migration created ext_sample table.
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO ext_sample (id, label) VALUES ('1', 'hello')`); err != nil {
		t.Fatalf("expected ext_sample table to exist: %v", err)
	}

	// Verify event subscription receives events.
	bus.Publish(ctx, domain.Event{Type: domain.EventIdeaCreated, EntityID: "i1"})
	if mod.eventsSeen != 1 {
		t.Fatalf("expected module to observe 1 event, got %d", mod.eventsSeen)
	}
}
