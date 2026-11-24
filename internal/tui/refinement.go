package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"personalcrm/internal/domain"
	"personalcrm/internal/ext"
)

type refinementStep int

const (
	refStepProjects refinementStep = iota
	refStepIdeas
	refStepRoutines
	refStepPlugins
	refStepSummary
)

type refinementSession struct {
	Run              domain.RefinementRun
	Step             refinementStep
	ProjectIDs       []string
	ProjectPos       int
	TaskCursor       int
	IdeaIDs          []string
	IdeaPos          int
	RoutineCursor    int
	PluginSteps      []ext.RefinementStepSpec
	PluginPos        int
	ProjectsReviewed map[string]bool
	IdeasReviewed    map[string]bool
	RoutinesChecked  bool
}

func (m *Model) startRefinement() {
	m.reload()

	cycleKey := m.refEval.CycleKey
	if cycleKey == "" {
		cycleKey = m.clock().Format("2006-01-02T15:04")
	}
	run, err := m.app.Store.StartRefinementRun(m.ctx, cycleKey)
	if err != nil {
		m.setErr(err)
		return
	}

	var projIDs []string
	for _, p := range m.projects {
		if p.IsRefinementEligible() {
			projIDs = append(projIDs, p.ID)
		}
	}
	var ideaIDs []string
	for _, it := range m.ideas {
		if it.IsRefinementEligible() {
			ideaIDs = append(ideaIDs, it.ID)
		}
	}

	var pluginSteps []ext.RefinementStepSpec
	if m.app.Registry != nil {
		pluginSteps = m.app.Registry.RefinementStepSpecs()
	}

	m.refinement = &refinementSession{
		Run:              run,
		Step:             refStepProjects,
		ProjectIDs:       projIDs,
		IdeaIDs:          ideaIDs,
		PluginSteps:      pluginSteps,
		ProjectsReviewed: map[string]bool{},
		IdeasReviewed:    map[string]bool{},
	}
	m.setStatus("Started Weekly Refinement check-in")
}

func (m *Model) updateRefinement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.refinement
	if rs == nil {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.refinement = nil
		m.setStatus("Paused Weekly Refinement (progress saved to DB)")
		m.reload()
		return m, nil
	}

	switch rs.Step {
	case refStepProjects:
		return m.updateRefinementProjects(msg)
	case refStepIdeas:
		return m.updateRefinementIdeas(msg)
	case refStepRoutines:
		return m.updateRefinementRoutines(msg)
	case refStepPlugins:
		switch msg.String() {
		case "right", "enter", "n":
			if rs.PluginPos < len(rs.PluginSteps)-1 {
				rs.PluginPos++
			} else {
				rs.Step = refStepSummary
			}
		case "left":
			if rs.PluginPos > 0 {
				rs.PluginPos--
			} else {
				rs.Step = refStepRoutines
			}
		}
		return m, nil
	case refStepSummary:
		switch msg.String() {
		case "left":
			if len(rs.PluginSteps) > 0 {
				rs.Step = refStepPlugins
			} else {
				rs.Step = refStepRoutines
			}
		case "enter", "c":
			now := m.clock().UTC()
			rs.Run.CompletedAt = &now
			rs.Run.ProjectsReviewed = len(rs.ProjectsReviewed)
			rs.Run.IdeasReviewed = len(rs.IdeasReviewed)
			if rs.RoutinesChecked {
				rs.Run.RoutinesReviewed = len(m.activeRoutines())
			}
			if err := m.app.Store.CompleteRefinementRun(m.ctx, &rs.Run); err != nil {
				m.setErr(err)
				return m, nil
			}
			m.publish(domain.EventRefinementCompleted, domain.EntityType("refinement"), rs.Run.ID)
			m.refinement = nil
			m.reload()
			m.setStatus("Weekly Refinement complete! Banner cleared.")
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) updateRefinementProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.refinement
	if len(rs.ProjectIDs) == 0 {
		if msg.String() == "right" || msg.String() == "enter" || msg.String() == "n" {
			rs.Step = refStepIdeas
		}
		return m, nil
	}

	proj, ok := m.findProjectByID(rs.ProjectIDs[rs.ProjectPos])
	if !ok {
		rs.Step = refStepIdeas
		return m, nil
	}
	tasks := m.tasksByProj[proj.ID]
	if rs.TaskCursor >= len(tasks) && len(tasks) > 0 {
		rs.TaskCursor = len(tasks) - 1
	}

	switch msg.String() {
	case "j", "down":
		if rs.TaskCursor < len(tasks)-1 {
			rs.TaskCursor++
		}
	case "k", "up":
		if rs.TaskCursor > 0 {
			rs.TaskCursor--
		}
	case " ":
		if len(tasks) > 0 && rs.TaskCursor < len(tasks) {
			t := tasks[rs.TaskCursor]
			if t.Status == domain.TaskDone {
				t.Status = domain.TaskOpen
			} else {
				t.Status = domain.TaskDone
			}
			if err := m.app.Store.UpdateTask(m.ctx, &t); err != nil {
				m.setErr(err)
			} else {
				rs.ProjectsReviewed[proj.ID] = true
				m.reload()
			}
		}
	case "s":
		proj.Status = nextProjectStatus(proj.Status)
		if err := m.app.Store.UpdateProject(m.ctx, &proj); err != nil {
			m.setErr(err)
		} else {
			rs.ProjectsReviewed[proj.ID] = true
			m.publish(domain.EventProjectStatusChanged, domain.EntityProject, proj.ID)
			m.setStatus(fmt.Sprintf("%q → %s", proj.Title, proj.Status))
			m.reload()
		}
	case "t":
		rs.ProjectsReviewed[proj.ID] = true
		m.openNewTaskModal(proj.ID, proj.Title)
	case "l":
		rs.ProjectsReviewed[proj.ID] = true
		m.openAddLogModal(domain.EntityProject, proj.ID, proj.Title)
	case "right", "enter", "n":
		rs.ProjectsReviewed[proj.ID] = true
		rs.TaskCursor = 0
		if rs.ProjectPos < len(rs.ProjectIDs)-1 {
			rs.ProjectPos++
		} else {
			rs.Step = refStepIdeas
		}
	case "left":
		rs.TaskCursor = 0
		if rs.ProjectPos > 0 {
			rs.ProjectPos--
		}
	}
	return m, nil
}

func (m *Model) updateRefinementIdeas(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.refinement
	if len(rs.IdeaIDs) == 0 {
		switch msg.String() {
		case "right", "enter", "n":
			rs.Step = refStepRoutines
		case "left":
			rs.Step = refStepProjects
		}
		return m, nil
	}

	idea, ok := m.findIdeaByID(rs.IdeaIDs[rs.IdeaPos])
	if !ok {
		rs.Step = refStepRoutines
		return m, nil
	}

	switch msg.String() {
	case "s":
		idea.Status = nextIdeaStatus(idea.Status)
		if err := m.app.Store.UpdateIdea(m.ctx, &idea); err != nil {
			m.setErr(err)
		} else {
			rs.IdeasReviewed[idea.ID] = true
			m.setStatus(fmt.Sprintf("Idea %q → %s", idea.Title, idea.Status))
			m.reload()
		}
	case "P":
		rs.IdeasReviewed[idea.ID] = true
		m.openPromoteIdeaModal(idea)
	case "right", "enter", "n":
		rs.IdeasReviewed[idea.ID] = true
		if rs.IdeaPos < len(rs.IdeaIDs)-1 {
			rs.IdeaPos++
		} else {
			rs.Step = refStepRoutines
		}
	case "left":
		if rs.IdeaPos > 0 {
			rs.IdeaPos--
		} else {
			rs.Step = refStepProjects
		}
	}
	return m, nil
}

func (m *Model) updateRefinementRoutines(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rs := m.refinement
	active := m.activeRoutines()
	switch msg.String() {
	case "j", "down":
		if rs.RoutineCursor < len(active)-1 {
			rs.RoutineCursor++
		}
	case "k", "up":
		if rs.RoutineCursor > 0 {
			rs.RoutineCursor--
		}
	case " ":
		if rs.RoutineCursor < len(active) {
			m.toggleRoutineCycle(active[rs.RoutineCursor])
			rs.RoutinesChecked = true
		}
	case "a":
		if rs.RoutineCursor < len(active) {
			r := active[rs.RoutineCursor]
			r.Active = false
			if err := m.app.Store.UpdateRoutine(m.ctx, &r); err != nil {
				m.setErr(err)
			} else {
				rs.RoutinesChecked = true
				m.setStatus(fmt.Sprintf("Paused routine %q", r.Title))
				m.reload()
			}
		}
	case "right", "enter", "n":
		rs.RoutinesChecked = true
		if len(rs.PluginSteps) > 0 {
			rs.Step = refStepPlugins
		} else {
			rs.Step = refStepSummary
		}
	case "left":
		rs.Step = refStepIdeas
	}
	return m, nil
}

func (m *Model) renderRefinement() string {
	rs := m.refinement
	if rs == nil {
		return ""
	}
	w := m.width - 4
	if w < 68 {
		w = 68
	}

	steps := []string{"1. Projects & Tasks", "2. Ideas Triage", "3. Routines Check"}
	if len(rs.PluginSteps) > 0 {
		steps = append(steps, "4. Plugins", "5. Summary")
	} else {
		steps = append(steps, "4. Summary")
	}

	var stepBar []string
	activeIdx := int(rs.Step)
	if len(rs.PluginSteps) == 0 && rs.Step == refStepSummary {
		activeIdx = 3
	}
	for i, lbl := range steps {
		if i == activeIdx {
			stepBar = append(stepBar, styleTabActive.Render(lbl))
		} else {
			stepBar = append(stepBar, styleTabInactive.Render(lbl))
		}
	}

	var b strings.Builder
	b.WriteString(styleTitle.Render("⚡ Weekly Refinement") + "   " + strings.Join(stepBar, " ") + "\n\n")

	switch rs.Step {
	case refStepProjects:
		if len(rs.ProjectIDs) == 0 {
			b.WriteString(styleMuted.Render("No active/waiting projects to review. Press [Enter] or [→] to continue to Ideas."))
		} else if p, ok := m.findProjectByID(rs.ProjectIDs[rs.ProjectPos]); ok {
			b.WriteString(fmt.Sprintf("Project %d of %d:  %s  %s  %s\n",
				rs.ProjectPos+1, len(rs.ProjectIDs), styleSelected.Render(p.Title), renderStatusPill(string(p.Status)), renderPriorityPill(p.Priority)))
			if p.Goal != "" {
				b.WriteString(styleMuted.Render("Goal: ") + p.Goal + "\n")
			}
			tasks := m.tasksByProj[p.ID]
			b.WriteString("\n" + styleSectionHeader.Render("Tasks:") + "\n")
			if len(tasks) == 0 {
				b.WriteString("  " + styleMuted.Render("(no tasks — press [t] to add a next action)") + "\n")
			} else {
				for i, t := range tasks {
					check := "[ ]"
					if t.Status == domain.TaskDone {
						check = "[x]"
					}
					line := fmt.Sprintf("%s %s %s", check, t.Title, renderStatusPill(string(t.Status)))
					if i == rs.TaskCursor {
						b.WriteString(styleSelected.Render("▸ "+line) + "\n")
					} else {
						b.WriteString("  " + line + "\n")
					}
				}
			}
			b.WriteString("\n" + styleMuted.Render("[Space] Toggle task  [t] Add task  [s] Cycle project status  [l] Log progress  [→/Enter] Next"))
		}

	case refStepIdeas:
		if len(rs.IdeaIDs) == 0 {
			b.WriteString(styleMuted.Render("No inbox/exploring ideas to triage! Press [Enter] or [→] to review Routines."))
		} else if it, ok := m.findIdeaByID(rs.IdeaIDs[rs.IdeaPos]); ok {
			b.WriteString(fmt.Sprintf("Idea %d of %d:  %s  %s\n",
				rs.IdeaPos+1, len(rs.IdeaIDs), styleSelected.Render(it.Title), renderStatusPill(string(it.Status))))
			if it.Description != "" {
				b.WriteString("\n" + truncate(it.Description, w*2) + "\n")
			}
			b.WriteString("\n" + styleMuted.Render("[s] Cycle status (exploring/ready/parked/discarded)   [P] Promote to Project   [→/Enter] Next"))
		}

	case refStepRoutines:
		active := m.activeRoutines()
		now := m.clock()
		b.WriteString(styleSectionHeader.Render("Review Active Routines") + "\n\n")
		if len(active) == 0 {
			b.WriteString(styleMuted.Render("No active routines configured. Press [→/Enter] to finish."))
		} else {
			for i, r := range active {
				done, info, _ := domain.IsCompletedForCycle(r, now)
				mark := "[ ]"
				if !info.IsDue {
					mark = "[.]"
				} else if done {
					mark = "[x]"
				}
				line := fmt.Sprintf("%s %-28s (%s)", mark, truncate(r.Title, 28), formatSchedule(r))
				if i == rs.RoutineCursor {
					b.WriteString(styleSelected.Render("▸ "+line) + "\n")
				} else {
					b.WriteString("  " + line + "\n")
				}
			}
			b.WriteString("\n" + styleMuted.Render("[Space] Check off due routine   [a] Pause routine   [→/Enter] Continue to Summary"))
		}

	case refStepPlugins:
		sp := rs.PluginSteps[rs.PluginPos]
		b.WriteString(fmt.Sprintf("Plugin Step (%d/%d): %s\n\n%s\n\n",
			rs.PluginPos+1, len(rs.PluginSteps), styleSelected.Render(sp.Title), sp.Summary))
		b.WriteString(styleMuted.Render("[→/Enter] Continue"))

	case refStepSummary:
		b.WriteString(styleSectionHeader.Render("Refinement Summary") + "\n\n")
		b.WriteString(fmt.Sprintf("  Projects reviewed: %d\n", len(rs.ProjectsReviewed)))
		b.WriteString(fmt.Sprintf("  Ideas triaged:     %d\n", len(rs.IdeasReviewed)))
		routinesCount := 0
		if rs.RoutinesChecked {
			routinesCount = len(m.activeRoutines())
		}
		b.WriteString(fmt.Sprintf("  Routines checked:  %d\n\n", routinesCount))
		b.WriteString(styleToast.Render("Press [Enter] to complete Weekly Refinement and clear the Dashboard banner."))
	}

	return stylePaneFocused.Width(w).Render(b.String())
}

func (m *Model) findProjectByID(id string) (domain.Project, bool) {
	for _, p := range m.projects {
		if p.ID == id {
			return p, true
		}
	}
	return domain.Project{}, false
}

func (m *Model) findIdeaByID(id string) (domain.Idea, bool) {
	for _, it := range m.ideas {
		if it.ID == id {
			return it, true
		}
	}
	return domain.Idea{}, false
}

func (m *Model) activeRoutines() []domain.Routine {
	var out []domain.Routine
	for _, r := range m.routines {
		if r.Active {
			out = append(out, r)
		}
	}
	return out
}
