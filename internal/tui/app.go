package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"personalcrm/internal/cli"
	"personalcrm/internal/config"
	"personalcrm/internal/domain"
	"personalcrm/internal/editor"
	"personalcrm/internal/ext"
	"personalcrm/internal/storage"
)

type tabID int

const (
	tabDashboard tabID = iota
	tabProjects
	tabIdeas
	tabHobbies
	tabRoutines
	tabKnowledge
	tabPluginBase // >= tabPluginBase maps into m.pluginTabs[tab - tabPluginBase]
)

type editorFinishedMsg struct {
	TempPath string
	Original string
	OnSave   func(updated string) error
	Err      error
}

type pluginTab struct {
	Spec  ext.TUIViewSpec
	Model tea.Model
}

// Model is the root Bubble Tea model for PersonalCRM.
type Model struct {
	ctx      context.Context
	app      *cli.App
	clock    func() time.Time
	width    int
	height   int
	active   tabID
	showHelp bool

	// Data snapshot from SQLite.
	projects    []domain.Project
	tasksByProj map[string][]domain.Task
	ideas       []domain.Idea
	hobbies     []domain.Hobby
	routines    []domain.Routine
	notes       []domain.Note
	resources   []domain.Resource
	refEval     domain.RefinementEvaluation

	// Per-tab cursors & sub-focus state.
	dashRoutineIdx int
	projIdx        int
	projTaskIdx    int
	projFocusTasks bool
	projFilter     int // 0=active+waiting, 1=all, 2=on_hold, 3=closed
	ideaIdx        int
	ideaFilter     int // 0=active(inbox/exploring/ready), 1=parked, 2=all
	hobbyIdx       int
	routineIdx     int
	knowledgeMode  int // 0=notes, 1=resources
	noteIdx        int
	resIdx         int

	// Pluggable UI tabs from ext.Registry.
	pluginTabs []pluginTab

	// Active modal overlay & Weekly Refinement wizard.
	modal      *activeModal
	refinement *refinementSession

	// Feedback line.
	statusMsg string
	errMsg    string
}

// New constructs an initialized TUI root Model backed by app's Store, Bus, Config, and Registry.
func New(ctx context.Context, app *cli.App) *Model {
	m := &Model{
		ctx:         ctx,
		app:         app,
		clock:       time.Now,
		width:       110,
		height:      34,
		active:      tabDashboard,
		tasksByProj: map[string][]domain.Task{},
	}

	if app.Registry != nil {
		for _, spec := range app.Registry.TUIViewSpecs() {
			var tm tea.Model
			if spec.Factory != nil {
				if raw, ok := spec.Factory().(tea.Model); ok {
					tm = raw
				}
			}
			m.pluginTabs = append(m.pluginTabs, pluginTab{Spec: spec, Model: tm})
		}
	}

	m.reload()
	return m
}

// SetClock overrides the wall clock (useful for deterministic automated tests).
func (m *Model) SetClock(fn func() time.Time) {
	if fn != nil {
		m.clock = fn
		m.reload()
	}
}

// Run starts the interactive full-screen Bubble Tea TUI program.
func Run(ctx context.Context, app *cli.App) error {
	m := New(ctx, app)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case editorFinishedMsg:
		defer os.Remove(msg.TempPath)
		if msg.Err != nil {
			m.setErr(fmt.Errorf("editor failed: %w", msg.Err))
			return m, nil
		}
		raw, err := os.ReadFile(msg.TempPath)
		if err != nil {
			m.setErr(fmt.Errorf("read edited file: %w", err))
			return m, nil
		}
		updated := strings.ReplaceAll(string(raw), "\r\n", "\n")
		orig := strings.ReplaceAll(msg.Original, "\r\n", "\n")
		if updated == orig {
			m.setStatus("No changes saved")
			return m, nil
		}
		if msg.OnSave != nil {
			if err := msg.OnSave(updated); err != nil {
				m.setErr(err)
			} else {
				m.setStatus("Saved changes from external editor")
				m.reload()
			}
		}
		return m, nil

	case tea.KeyMsg:
		m.errMsg = ""

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Active modal captures all keystrokes first.
		if m.modal != nil {
			return m.updateModal(msg)
		}

		// Help overlay dismissal.
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
				m.showHelp = false
			}
			return m, nil
		}

		// Guided Weekly Refinement wizard takes priority when open.
		if m.refinement != nil {
			return m.updateRefinement(msg)
		}

		// Global shortcuts.
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "r":
			m.reload()
			m.setStatus("Refreshed from database")
			return m, nil
		case "R":
			m.startRefinement()
			return m, nil
		case "S":
			if m.refEval.Due {
				hours := m.app.Config.Refinement.SnoozeHours
				if hours < 1 {
					hours = 24
				}
				until := m.clock().UTC().Add(time.Duration(hours) * time.Hour)
				if err := m.app.Store.SnoozeRefinement(m.ctx, until); err != nil {
					m.setErr(err)
				} else {
					m.setStatus(fmt.Sprintf("Weekly Refinement snoozed for %dh", hours))
					m.reload()
				}
			}
			return m, nil
		case "1":
			m.active = tabDashboard
			return m, nil
		case "2":
			m.active = tabProjects
			return m, nil
		case "3":
			m.active = tabIdeas
			return m, nil
		case "4":
			m.active = tabHobbies
			return m, nil
		case "5":
			m.active = tabRoutines
			return m, nil
		case "6":
			m.active = tabKnowledge
			return m, nil
		case "tab":
			total := int(tabPluginBase) + len(m.pluginTabs)
			m.active = tabID((int(m.active) + 1) % total)
			return m, nil
		case "shift+tab":
			total := int(tabPluginBase) + len(m.pluginTabs)
			m.active = tabID((int(m.active) - 1 + total) % total)
			return m, nil
		}

		// Numeric shortcuts 7..9 for dynamic plugin tabs.
		if len(msg.String()) == 1 && msg.String()[0] >= '7' && msg.String()[0] <= '9' {
			idx := int(msg.String()[0] - '7')
			if idx < len(m.pluginTabs) {
				m.active = tabPluginBase + tabID(idx)
				return m, nil
			}
		}

		return m.updateActiveTab(msg)
	}

	return m, nil
}

func (m *Model) View() string {
	var parts []string

	// Top header + tabs bar.
	parts = append(parts, m.renderHeader())

	// Prominent non-blocking Weekly Refinement banner when due (on normal tabs).
	if m.refinement == nil && m.refEval.Due {
		banner := styleBanner.Render("⚡ WEEKLY REFINEMENT DUE — Press [R] to start check-in, [S] to snooze")
		parts = append(parts, banner)
	}

	// Main content body.
	switch {
	case m.modal != nil:
		parts = append(parts, m.renderModal())
	case m.showHelp:
		parts = append(parts, m.renderHelpOverlay())
	case m.refinement != nil:
		parts = append(parts, m.renderRefinement())
	default:
		parts = append(parts, m.renderActiveTab())
	}

	// Feedback line + footer.
	if m.errMsg != "" {
		parts = append(parts, styleError.Render("Error: "+m.errMsg))
	} else if m.statusMsg != "" {
		parts = append(parts, styleToast.Render("✓ "+m.statusMsg))
	}
	parts = append(parts, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *Model) renderHeader() string {
	tabs := []struct {
		id    tabID
		label string
	}{
		{tabDashboard, "1 Dashboard"},
		{tabProjects, "2 Projects"},
		{tabIdeas, "3 Ideas"},
		{tabHobbies, "4 Hobbies"},
		{tabRoutines, "5 Routines"},
		{tabKnowledge, "6 Notes & Links"},
	}
	for i, pt := range m.pluginTabs {
		tabs = append(tabs, struct {
			id    tabID
			label string
		}{
			id:    tabPluginBase + tabID(i),
			label: fmt.Sprintf("%d %s", 7+i, pt.Spec.Title),
		})
	}

	var rendered []string
	rendered = append(rendered, styleTitle.Render("PersonalCRM")+" ")
	for _, t := range tabs {
		if m.refinement == nil && m.active == t.id {
			rendered = append(rendered, styleTabActive.Render(t.label))
		} else {
			rendered = append(rendered, styleTabInactive.Render(t.label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, rendered...)
}

func (m *Model) renderFooter() string {
	if m.refinement != nil {
		return styleFooter.Render("Refinement: [←/→] Step/Next  [Space] Toggle  [s] Status  [t] Task  [l] Log  [P] Promote  [Esc] Exit")
	}
	common := "[1-6/Tab] Switch  [R] Refinement  [r] Refresh  [?] Help  [q] Quit"
	switch m.active {
	case tabDashboard:
		return styleFooter.Render("Dashboard: [j/k] Select Routine  [Space] Check off   |   " + common)
	case tabProjects:
		return styleFooter.Render("Projects: [←/→] Pane  [n] New  [t] Task  [Space] Done  [s] Status  [p] Prio  [l] Log  [c] Link  [e] $EDITOR  [f] Filter")
	case tabIdeas:
		return styleFooter.Render("Ideas: [n] New  [s] Status  [P] Promote→Project  [c] Link  [g] Tag  [e] $EDITOR  [f] Filter  [d] Del")
	case tabHobbies:
		return styleFooter.Render("Hobbies: [n] New  [s] Status  [l] Log  [c] Link  [g] Tag  [e] $EDITOR  [d] Del")
	case tabRoutines:
		return styleFooter.Render("Routines: [Space] Check off  [n] New  [a] Active/Pause  [m] Reminder  [d] Del")
	case tabKnowledge:
		return styleFooter.Render("Notes & Links: [v/←/→] Switch Notes/Links  [n] New  [e] $EDITOR  [s] Status  [c] Link  [g] Tag  [d] Del")
	default:
		return styleFooter.Render(common)
	}
}

func (m *Model) renderHelpOverlay() string {
	help := strings.Join([]string{
		styleTitle.Render("PersonalCRM Keyboard Shortcuts"),
		"",
		styleSectionHeader.Render("Global"),
		"  1..6 (7..9 plugins)  Jump directly to tab",
		"  Tab / Shift+Tab      Cycle tabs",
		"  R                    Start Weekly Refinement wizard",
		"  S                    Snooze due Weekly Refinement",
		"  r                    Reload data from SQLite (picks up CLI updates)",
		"  ?                    Toggle this help",
		"  q / Ctrl+C           Quit TUI (tray daemon keeps running in Phase 5)",
		"",
		styleSectionHeader.Render("Editing & Linking"),
		"  n                    Create new entity in current tab",
		"  e / E                Edit description/note in external $EDITOR",
		"  s / p                Cycle status / priority",
		"  t                    Add task to selected Project",
		"  l                    Add dated progress/reflection log",
		"  c / K                Link standalone Note or Resource polymorphically",
		"  g                    Add comma-separated tags",
		"  P (in Ideas)         Promote Idea to Project (carries over links & tags)",
		"  Space / Enter        Toggle Task or Routine completion",
		"  d                    Delete selected item (with confirmation)",
		"",
		styleMuted.Render("Press [?], [Esc], or [q] to close help."),
	}, "\n")
	return styleModal.Render(help)
}

// reload refreshes all in-memory slices from SQLite and clamps cursors safely.
func (m *Model) reload() {
	if m.app == nil || m.app.Store == nil {
		return
	}
	st := m.app.Store

	m.projects, _ = st.ListProjects(m.ctx, storage.ProjectFilter{})
	m.tasksByProj = map[string][]domain.Task{}
	for _, p := range m.projects {
		tasks, _ := st.ListTasksByProject(m.ctx, p.ID, true)
		m.tasksByProj[p.ID] = tasks
	}
	m.ideas, _ = st.ListIdeas(m.ctx)
	m.hobbies, _ = st.ListHobbies(m.ctx)
	m.routines, _ = st.ListRoutines(m.ctx, false)
	m.notes, _ = st.ListNotes(m.ctx, "")
	m.resources, _ = st.ListResources(m.ctx, "")

	m.refEval = m.evaluateRefinementNow()

	m.clampCursors()
}

func (m *Model) evaluateRefinementNow() domain.RefinementEvaluation {
	cfg := m.app.Config.Refinement
	wd, err := config.ParseWeekday(cfg.Weekday)
	if err != nil {
		wd = time.Sunday
	}
	hr, min, err := config.ParseClockTime(cfg.Time)
	if err != nil {
		hr, min = 18, 0
	}
	refState, err := m.app.Store.GetRefinementState(m.ctx)
	if err != nil {
		return domain.RefinementEvaluation{}
	}
	return domain.EvaluateRefinement(domain.RefinementSchedule{
		Enabled: cfg.Enabled,
		Weekday: wd,
		Hour:    hr,
		Minute:  min,
	}, refState, m.clock())
}

func (m *Model) clampCursors() {
	clamp := func(idx, length int) int {
		if length <= 0 {
			return 0
		}
		if idx < 0 {
			return 0
		}
		if idx >= length {
			return length - 1
		}
		return idx
	}
	m.dashRoutineIdx = clamp(m.dashRoutineIdx, len(m.dueRoutines()))
	m.projIdx = clamp(m.projIdx, len(m.filteredProjects()))
	if p, ok := m.selectedProject(); ok {
		m.projTaskIdx = clamp(m.projTaskIdx, len(m.tasksByProj[p.ID]))
	} else {
		m.projTaskIdx = 0
	}
	m.ideaIdx = clamp(m.ideaIdx, len(m.filteredIdeas()))
	m.hobbyIdx = clamp(m.hobbyIdx, len(m.hobbies))
	m.routineIdx = clamp(m.routineIdx, len(m.routines))
	m.noteIdx = clamp(m.noteIdx, len(m.notes))
	m.resIdx = clamp(m.resIdx, len(m.resources))
}

func (m *Model) openExternalEditor(initialContent string, onSave func(string) error) tea.Cmd {
	tmpPath, err := editor.PrepareTempFile("pcrm-edit-", initialContent)
	if err != nil {
		m.setErr(err)
		return nil
	}
	cmd := editor.BuildCommand(m.ctx, m.app.Config.Editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorFinishedMsg{
			TempPath: tmpPath,
			Original: initialContent,
			OnSave:   onSave,
			Err:      err,
		}
	})
}

func (m *Model) publish(evType domain.EventType, entType domain.EntityType, id string) {
	if m.app == nil || m.app.Bus == nil {
		return
	}
	_ = m.app.Bus.Publish(m.ctx, domain.Event{
		Type:       evType,
		EntityType: entType,
		EntityID:   id,
		OccurredAt: m.clock().UTC(),
	})
}

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.errMsg = ""
}

func (m *Model) setErr(err error) {
	if err != nil {
		m.errMsg = err.Error()
		m.statusMsg = ""
	}
}

func (m *Model) paneWidths() (leftW, rightW int) {
	w := m.width
	if w < 70 {
		w = 70
	}
	leftW = w * 38 / 100
	if leftW < 28 {
		leftW = 28
	}
	rightW = w - leftW - 6
	if rightW < 34 {
		rightW = 34
	}
	return leftW, rightW
}
