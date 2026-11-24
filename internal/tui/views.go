package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"personalcrm/internal/domain"
)

func (m *Model) updateActiveTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.active {
	case tabDashboard:
		return m.updateDashboard(msg)
	case tabProjects:
		return m.updateProjects(msg)
	case tabIdeas:
		return m.updateIdeas(msg)
	case tabHobbies:
		return m.updateHobbies(msg)
	case tabRoutines:
		return m.updateRoutines(msg)
	case tabKnowledge:
		return m.updateKnowledge(msg)
	default:
		idx := int(m.active - tabPluginBase)
		if idx >= 0 && idx < len(m.pluginTabs) && m.pluginTabs[idx].Model != nil {
			updated, cmd := m.pluginTabs[idx].Model.Update(msg)
			m.pluginTabs[idx].Model = updated
			return m, cmd
		}
		return m, nil
	}
}

func (m *Model) renderActiveTab() string {
	switch m.active {
	case tabDashboard:
		return m.renderDashboard()
	case tabProjects:
		return m.renderProjects()
	case tabIdeas:
		return m.renderIdeas()
	case tabHobbies:
		return m.renderHobbies()
	case tabRoutines:
		return m.renderRoutines()
	case tabKnowledge:
		return m.renderKnowledge()
	default:
		idx := int(m.active - tabPluginBase)
		if idx >= 0 && idx < len(m.pluginTabs) {
			pt := m.pluginTabs[idx]
			if pt.Model != nil {
				return pt.Model.View()
			}
			return stylePane.Render(fmt.Sprintf("%s\n\n%s", styleTitle.Render(pt.Spec.Title), styleMuted.Render("Plugin tab active.")))
		}
		return ""
	}
}

// --- 1. Dashboard ---

func (m *Model) dueRoutines() []domain.Routine {
	now := m.clock()
	var out []domain.Routine
	for _, r := range m.routines {
		if !r.Active {
			continue
		}
		if info, err := domain.ActiveCycle(r, now); err == nil && info.IsDue {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	due := m.dueRoutines()
	switch msg.String() {
	case "j", "down":
		if m.dashRoutineIdx < len(due)-1 {
			m.dashRoutineIdx++
		}
	case "k", "up":
		if m.dashRoutineIdx > 0 {
			m.dashRoutineIdx--
		}
	case " ", "enter":
		if m.dashRoutineIdx < len(due) {
			m.toggleRoutineCycle(due[m.dashRoutineIdx])
		}
	}
	return m, nil
}

func (m *Model) renderDashboard() string {
	leftW, rightW := m.paneWidths()
	now := m.clock()

	// Summary stats strip.
	activeProjs := 0
	for _, p := range m.projects {
		if p.IsRefinementEligible() {
			activeProjs++
		}
	}
	inboxIdeas := 0
	for _, it := range m.ideas {
		if it.IsRefinementEligible() {
			inboxIdeas++
		}
	}
	activeHobbies := 0
	for _, h := range m.hobbies {
		if h.Status == domain.HobbyActive {
			activeHobbies++
		}
	}
	due := m.dueRoutines()
	doneDue := 0
	for _, r := range due {
		if ok, _, _ := domain.IsCompletedForCycle(r, now); ok {
			doneDue++
		}
	}

	stats := styleMuted.Render(fmt.Sprintf(
		"Active Projects: %d   •   Ideas to Triage: %d   •   Active Hobbies: %d   •   Due Routines: %d/%d done",
		activeProjs, inboxIdeas, activeHobbies, doneDue, len(due),
	))

	// Left pane: Due Routines checklist.
	var lb strings.Builder
	writeStrings(&lb, styleSectionHeader.Render("Today's Due Routines"), "\n\n")
	if len(due) == 0 {
		lb.WriteString(styleMuted.Render("No routines due right now. Press [5] to add routines."))
	} else {
		for i, r := range due {
			done, _, _ := domain.IsCompletedForCycle(r, now)
			check := "[ ]"
			if done {
				check = "[x]"
			}
			line := fmt.Sprintf("%s %s %s", check, truncate(r.Title, leftW-16), styleMuted.Render("("+string(r.ScheduleType)+")"))
			if i == m.dashRoutineIdx {
				writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
			} else {
				writeStrings(&lb, "  ", line, "\n")
			}
		}
	}
	leftPane := stylePaneFocused.Width(leftW).Render(lb.String())

	// Right pane: Active Projects & Next Actions.
	var rb strings.Builder
	writeStrings(&rb, styleSectionHeader.Render("Active Projects & Open Next Actions"), "\n\n")
	shown := 0
	for _, p := range m.projects {
		if !p.IsRefinementEligible() {
			continue
		}
		shown++
		if shown > 8 {
			rb.WriteString(styleMuted.Render("... press [2] to view all projects"))
			break
		}
		fmt.Fprintf(&rb, "%s %s %s\n", renderPriorityPill(p.Priority), styleSelected.Render(truncate(p.Title, rightW-20)), renderStatusPill(string(p.Status)))
		openCount := 0
		for _, t := range m.tasksByProj[p.ID] {
			if !t.IsOpen() {
				continue
			}
			openCount++
			if openCount <= 3 {
				badges := renderPluginBadges(m.app.Registry.EntityBadges(m.ctx, domain.EntityTask, t.ID))
				fmt.Fprintf(&rb, "   • %s%s\n", truncate(t.Title, rightW-12), badges)
			}
		}
		if openCount == 0 {
			writeStrings(&rb, "   ", styleMuted.Render("(no open next actions)"), "\n")
		} else if openCount > 3 {
			writeStrings(&rb, "   ", styleMuted.Render(fmt.Sprintf("+%d more open tasks", openCount-3)), "\n")
		}
		rb.WriteString("\n")
	}
	if shown == 0 {
		rb.WriteString(styleMuted.Render("No active projects yet. Press [2] then [n] to create one, or [3] to promote an Idea."))
	}
	rightPane := stylePane.Width(rightW).Render(rb.String())

	return lipgloss.JoinVertical(lipgloss.Left, stats, lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane))
}

// --- 2. Projects & Tasks ---

func (m *Model) filteredProjects() []domain.Project {
	var out []domain.Project
	for _, p := range m.projects {
		switch m.projFilter {
		case 0: // active + waiting
			if p.IsRefinementEligible() {
				out = append(out, p)
			}
		case 1: // all
			out = append(out, p)
		case 2: // on_hold
			if p.Status == domain.ProjectOnHold {
				out = append(out, p)
			}
		case 3: // finished + rejected
			if p.Status == domain.ProjectFinished || p.Status == domain.ProjectRejected {
				out = append(out, p)
			}
		}
	}
	return out
}

func (m *Model) selectedProject() (domain.Project, bool) {
	list := m.filteredProjects()
	if len(list) == 0 || m.projIdx >= len(list) {
		return domain.Project{}, false
	}
	return list[m.projIdx], true
}

func (m *Model) updateProjects(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.filteredProjects()
	proj, hasProj := m.selectedProject()
	var tasks []domain.Task
	if hasProj {
		tasks = m.tasksByProj[proj.ID]
	}

	switch msg.String() {
	case "h", "left":
		m.projFocusTasks = false
	case "l", "right":
		if hasProj && msg.String() == "right" {
			m.projFocusTasks = true
		} else if hasProj && msg.String() == "l" {
			// 'l' adds a dated progress log on the selected project!
			m.openAddLogModal(domain.EntityProject, proj.ID, proj.Title)
		}
	case "j", "down":
		if m.projFocusTasks {
			if m.projTaskIdx < len(tasks)-1 {
				m.projTaskIdx++
			}
		} else if m.projIdx < len(list)-1 {
			m.projIdx++
			m.projTaskIdx = 0
		}
	case "k", "up":
		if msg.String() == "k" && !m.projFocusTasks && hasProj {
			// Wait: keep 'k' as vim up! Use 'L' or 'K' or check: let's check footer -> use 'up'/'down' + 'j'/'k' for navigation, and 'c' (connect/link) or 'K' for linking!
			if m.projIdx > 0 {
				m.projIdx--
				m.projTaskIdx = 0
			}
		} else if m.projFocusTasks {
			if m.projTaskIdx > 0 {
				m.projTaskIdx--
			}
		} else if m.projIdx > 0 {
			m.projIdx--
			m.projTaskIdx = 0
		}
	case "f":
		m.projFilter = (m.projFilter + 1) % 4
		m.projIdx = 0
		m.projTaskIdx = 0
		m.setStatus("Project filter: " + projectFilterLabel(m.projFilter))
	case "n":
		m.openNewProjectModal()
	case "t":
		if hasProj {
			m.openNewTaskModal(proj.ID, proj.Title)
		}
	case " ":
		if hasProj && len(tasks) > 0 && m.projTaskIdx < len(tasks) {
			t := tasks[m.projTaskIdx]
			if t.Status == domain.TaskDone {
				t.Status = domain.TaskOpen
			} else {
				t.Status = domain.TaskDone
			}
			if err := m.app.Store.UpdateTask(m.ctx, &t); err != nil {
				m.setErr(err)
			} else {
				if t.Status == domain.TaskDone {
					m.publish(domain.EventTaskCompleted, domain.EntityTask, t.ID)
				}
				m.setStatus(fmt.Sprintf("Task %q → %s", t.Title, t.Status))
				m.reload()
			}
		}
	case "s":
		if hasProj {
			if m.projFocusTasks && len(tasks) > 0 && m.projTaskIdx < len(tasks) {
				t := tasks[m.projTaskIdx]
				t.Status = nextTaskStatus(t.Status)
				if err := m.app.Store.UpdateTask(m.ctx, &t); err != nil {
					m.setErr(err)
				} else {
					m.setStatus(fmt.Sprintf("Task status → %s", t.Status))
					m.reload()
				}
			} else {
				proj.Status = nextProjectStatus(proj.Status)
				if err := m.app.Store.UpdateProject(m.ctx, &proj); err != nil {
					m.setErr(err)
				} else {
					m.publish(domain.EventProjectStatusChanged, domain.EntityProject, proj.ID)
					m.setStatus(fmt.Sprintf("Project %q → %s", proj.Title, proj.Status))
					m.reload()
				}
			}
		}
	case "p":
		if hasProj {
			proj.Priority = nextPriority(proj.Priority)
			if err := m.app.Store.UpdateProject(m.ctx, &proj); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Project priority → %s", proj.Priority))
				m.reload()
			}
		}
	case "g":
		if hasProj {
			m.openAddTagModal(domain.EntityProject, proj.ID, proj.Title)
		}
	case "K", "c":
		if hasProj {
			m.openLinkPickerModal(domain.EntityProject, proj.ID, proj.Title)
		}
	case "e":
		if hasProj {
			if m.projFocusTasks && len(tasks) > 0 && m.projTaskIdx < len(tasks) {
				taskCopy := tasks[m.projTaskIdx]
				return m, m.openExternalEditor(taskCopy.Description, func(updated string) error {
					taskCopy.Description = updated
					return m.app.Store.UpdateTask(m.ctx, &taskCopy)
				})
			}
			projCopy := proj
			return m, m.openExternalEditor(projCopy.Description, func(updated string) error {
				projCopy.Description = updated
				return m.app.Store.UpdateProject(m.ctx, &projCopy)
			})
		}
	case "d":
		if hasProj {
			if m.projFocusTasks && len(tasks) > 0 && m.projTaskIdx < len(tasks) {
				t := tasks[m.projTaskIdx]
				m.modal = newConfirmModal("Delete Task", fmt.Sprintf("Delete task %q?", t.Title), func() error {
					if err := m.app.Store.DeleteTask(m.ctx, t.ID); err != nil {
						return err
					}
					m.setStatus("Deleted task")
					return nil
				})
			} else {
				p := proj
				m.modal = newConfirmModal("Delete Project", fmt.Sprintf("Delete project %q and its tasks?", p.Title), func() error {
					if err := m.app.Store.DeleteProject(m.ctx, p.ID); err != nil {
						return err
					}
					m.setStatus("Deleted project")
					return nil
				})
			}
		}
	}
	return m, nil
}

func (m *Model) renderProjects() string {
	leftW, rightW := m.paneWidths()
	list := m.filteredProjects()

	var lb strings.Builder
	writeStrings(&lb, styleSectionHeader.Render(fmt.Sprintf("Projects (%s)", projectFilterLabel(m.projFilter))), "\n\n")
	if len(list) == 0 {
		lb.WriteString(styleMuted.Render("No projects in this filter.\nPress [n] to create or [f] to switch filter."))
	} else {
		for i, p := range list {
			line := fmt.Sprintf("%s %s %s", renderPriorityPill(p.Priority), truncate(p.Title, leftW-18), renderStatusPill(string(p.Status)))
			if i == m.projIdx {
				writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
			} else {
				writeStrings(&lb, "  ", line, "\n")
			}
		}
	}

	var rb strings.Builder
	if p, ok := m.selectedProject(); !ok {
		rb.WriteString(styleMuted.Render("Select a project to inspect tasks, logs, and links."))
	} else {
		fmt.Fprintf(&rb, "%s  %s  %s\n", styleTitle.Render(p.Title), renderStatusPill(string(p.Status)), renderPriorityPill(p.Priority))
		if p.Goal != "" {
			writeStrings(&rb, styleMuted.Render("Goal: "), p.Goal, "\n")
		}
		if tags, _ := m.app.Store.ListTagsFor(m.ctx, domain.EntityProject, p.ID); len(tags) > 0 {
			writeStrings(&rb, renderTagsLine(tags), "\n")
		}
		if p.Description != "" {
			writeStrings(&rb, "\n", truncate(p.Description, rightW*2), "\n")
		}

		// Plugin detail sections (e.g. Customer / People when ext:people is enabled).
		for _, sec := range m.app.Registry.EntityDetailSections(m.ctx, domain.EntityProject, p.ID) {
			writeStrings(&rb, "\n", styleSectionHeader.Render(sec.Title), "\n")
			for _, ln := range sec.Lines {
				writeStrings(&rb, "  ", ln, "\n")
			}
		}

		// Tasks list with optional @handle plugin badges.
		tasks := m.tasksByProj[p.ID]
		writeStrings(&rb, "\n", styleSectionHeader.Render(fmt.Sprintf("Tasks (%d) — [→] focus, [t] add, [Space] done, [e] $EDITOR", len(tasks))), "\n")
		if len(tasks) == 0 {
			writeStrings(&rb, "  ", styleMuted.Render("(no tasks yet)"), "\n")
		} else {
			for i, t := range tasks {
				check := "[ ]"
				if t.Status == domain.TaskDone {
					check = "[x]"
				} else if t.Status == domain.TaskBlocked {
					check = "[!]"
				} else if t.Status == domain.TaskInProgress {
					check = "[~]"
				}
				badges := renderPluginBadges(m.app.Registry.EntityBadges(m.ctx, domain.EntityTask, t.ID))
				dueStr := ""
				if t.DueAt != nil {
					dueStr = styleMuted.Render(" due:" + t.DueAt.Format("2006-01-02"))
				}
				line := fmt.Sprintf("%s %s%s%s", check, truncate(t.Title, rightW-22), badges, dueStr)
				if m.projFocusTasks && i == m.projTaskIdx {
					writeStrings(&rb, styleSelected.Render("▸ "+line), "\n")
				} else {
					writeStrings(&rb, "  ", line, "\n")
				}
			}
		}

		// Dated Progress Logs.
		logs, _ := m.app.Store.ListProgressLogs(m.ctx, domain.EntityProject, p.ID)
		writeStrings(&rb, "\n", styleSectionHeader.Render(fmt.Sprintf("Progress Log (%d) — [l] add entry", len(logs))), "\n")
		if len(logs) == 0 {
			writeStrings(&rb, "  ", styleMuted.Render("(no logs yet)"), "\n")
		} else {
			for i, lg := range logs {
				if i >= 4 {
					break
				}
				fmt.Fprintf(&rb, "  %s %s\n", styleMuted.Render(lg.LoggedAt.Format("2006-01-02")), truncate(lg.Content, rightW-16))
			}
		}

		// Polymorphic Linked Notes & Resources.
		rb.WriteString(m.renderLinkedSummary(domain.EntityProject, p.ID, rightW))
	}

	leftStyle, rightStyle := stylePaneFocused, stylePane
	if m.projFocusTasks {
		leftStyle, rightStyle = stylePane, stylePaneFocused
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Width(leftW).Render(lb.String()), " ", rightStyle.Width(rightW).Render(rb.String()))
}

// --- 3. Ideas Tracker ---

func (m *Model) filteredIdeas() []domain.Idea {
	var out []domain.Idea
	for _, it := range m.ideas {
		switch m.ideaFilter {
		case 0: // active inbox/exploring/ready
			if it.IsRefinementEligible() {
				out = append(out, it)
			}
		case 1: // parked
			if it.Status == domain.IdeaParked {
				out = append(out, it)
			}
		case 2: // all
			out = append(out, it)
		}
	}
	return out
}

func (m *Model) selectedIdea() (domain.Idea, bool) {
	list := m.filteredIdeas()
	if len(list) == 0 || m.ideaIdx >= len(list) {
		return domain.Idea{}, false
	}
	return list[m.ideaIdx], true
}

func (m *Model) updateIdeas(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.filteredIdeas()
	idea, hasIdea := m.selectedIdea()

	switch msg.String() {
	case "j", "down":
		if m.ideaIdx < len(list)-1 {
			m.ideaIdx++
		}
	case "k", "up":
		if m.ideaIdx > 0 {
			m.ideaIdx--
		}
	case "f":
		m.ideaFilter = (m.ideaFilter + 1) % 3
		m.ideaIdx = 0
		m.setStatus("Idea filter: " + ideaFilterLabel(m.ideaFilter))
	case "n":
		m.openNewIdeaModal()
	case "s":
		if hasIdea {
			idea.Status = nextIdeaStatus(idea.Status)
			if err := m.app.Store.UpdateIdea(m.ctx, &idea); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Idea %q → %s", idea.Title, idea.Status))
				m.reload()
			}
		}
	case "P":
		if hasIdea {
			m.openPromoteIdeaModal(idea)
		}
	case "g":
		if hasIdea {
			m.openAddTagModal(domain.EntityIdea, idea.ID, idea.Title)
		}
	case "c", "K":
		if hasIdea {
			m.openLinkPickerModal(domain.EntityIdea, idea.ID, idea.Title)
		}
	case "e":
		if hasIdea {
			ideaCopy := idea
			return m, m.openExternalEditor(ideaCopy.Description, func(updated string) error {
				ideaCopy.Description = updated
				return m.app.Store.UpdateIdea(m.ctx, &ideaCopy)
			})
		}
	case "d":
		if hasIdea {
			it := idea
			m.modal = newConfirmModal("Delete Idea", fmt.Sprintf("Delete idea %q?", it.Title), func() error {
				if err := m.app.Store.DeleteIdea(m.ctx, it.ID); err != nil {
					return err
				}
				m.setStatus("Deleted idea")
				return nil
			})
		}
	}
	return m, nil
}

func (m *Model) renderIdeas() string {
	leftW, rightW := m.paneWidths()
	list := m.filteredIdeas()

	var lb strings.Builder
	writeStrings(&lb, styleSectionHeader.Render(fmt.Sprintf("Ideas Tracker (%s)", ideaFilterLabel(m.ideaFilter))), "\n\n")
	if len(list) == 0 {
		lb.WriteString(styleMuted.Render("No ideas here. Press [n] to capture one or [f] to switch filter."))
	} else {
		for i, it := range list {
			line := fmt.Sprintf("%s %s", truncate(it.Title, leftW-16), renderStatusPill(string(it.Status)))
			if i == m.ideaIdx {
				writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
			} else {
				writeStrings(&lb, "  ", line, "\n")
			}
		}
	}

	var rb strings.Builder
	if it, ok := m.selectedIdea(); !ok {
		rb.WriteString(styleMuted.Render("Select an idea to research, link notes/URLs, or promote [P] to a Project."))
	} else {
		fmt.Fprintf(&rb, "%s  %s\n", styleTitle.Render(it.Title), renderStatusPill(string(it.Status)))
		if it.Source != "" {
			writeStrings(&rb, styleMuted.Render("Source: "), it.Source, "\n")
		}
		if it.Excitement != nil {
			writeStrings(&rb, styleMuted.Render("Excitement: "), strings.Repeat("★", *it.Excitement), "\n")
		}
		if it.PromotedProjectID != "" {
			writeStrings(&rb, styleToast.Render("Promoted → Project "+it.PromotedProjectID), "\n")
		}
		if tags, _ := m.app.Store.ListTagsFor(m.ctx, domain.EntityIdea, it.ID); len(tags) > 0 {
			writeStrings(&rb, renderTagsLine(tags), "\n")
		}
		rb.WriteString("\n")
		if it.Description != "" {
			writeStrings(&rb, it.Description, "\n")
		} else {
			writeStrings(&rb, styleMuted.Render("(no description yet — press [e] to write in $EDITOR)"), "\n")
		}
		rb.WriteString(m.renderLinkedSummary(domain.EntityIdea, it.ID, rightW))
		writeStrings(&rb, "\n", styleMuted.Render("[P] Promote to Project   [c] Link Note/Resource   [g] Tag   [e] $EDITOR"))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, stylePaneFocused.Width(leftW).Render(lb.String()), " ", stylePane.Width(rightW).Render(rb.String()))
}

// --- 4. Hobbies ---

func (m *Model) selectedHobby() (domain.Hobby, bool) {
	if len(m.hobbies) == 0 || m.hobbyIdx >= len(m.hobbies) {
		return domain.Hobby{}, false
	}
	return m.hobbies[m.hobbyIdx], true
}

func (m *Model) updateHobbies(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	hobby, has := m.selectedHobby()
	switch msg.String() {
	case "j", "down":
		if m.hobbyIdx < len(m.hobbies)-1 {
			m.hobbyIdx++
		}
	case "k", "up":
		if m.hobbyIdx > 0 {
			m.hobbyIdx--
		}
	case "n":
		m.openNewHobbyModal()
	case "s":
		if has {
			hobby.Status = nextHobbyStatus(hobby.Status)
			if err := m.app.Store.UpdateHobby(m.ctx, &hobby); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Hobby %q → %s", hobby.Name, hobby.Status))
				m.reload()
			}
		}
	case "l":
		if has {
			m.openAddLogModal(domain.EntityHobby, hobby.ID, hobby.Name)
		}
	case "g":
		if has {
			m.openAddTagModal(domain.EntityHobby, hobby.ID, hobby.Name)
		}
	case "c", "K":
		if has {
			m.openLinkPickerModal(domain.EntityHobby, hobby.ID, hobby.Name)
		}
	case "e":
		if has {
			hCopy := hobby
			return m, m.openExternalEditor(hCopy.Description, func(updated string) error {
				hCopy.Description = updated
				return m.app.Store.UpdateHobby(m.ctx, &hCopy)
			})
		}
	case "d":
		if has {
			h := hobby
			m.modal = newConfirmModal("Delete Hobby", fmt.Sprintf("Delete hobby %q?", h.Name), func() error {
				if err := m.app.Store.DeleteHobby(m.ctx, h.ID); err != nil {
					return err
				}
				m.setStatus("Deleted hobby")
				return nil
			})
		}
	}
	return m, nil
}

func (m *Model) renderHobbies() string {
	leftW, rightW := m.paneWidths()

	var lb strings.Builder
	writeStrings(&lb, styleSectionHeader.Render("Hobbies & Ongoing Interests"), "\n\n")
	if len(m.hobbies) == 0 {
		lb.WriteString(styleMuted.Render("No hobbies tracked yet. Press [n] to add one."))
	} else {
		for i, h := range m.hobbies {
			line := fmt.Sprintf("%s %s", truncate(h.Name, leftW-16), renderStatusPill(string(h.Status)))
			if i == m.hobbyIdx {
				writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
			} else {
				writeStrings(&lb, "  ", line, "\n")
			}
		}
	}

	var rb strings.Builder
	if h, ok := m.selectedHobby(); !ok {
		rb.WriteString(styleMuted.Render("Select a hobby to view reflections, progress logs, and links."))
	} else {
		fmt.Fprintf(&rb, "%s  %s\n", styleTitle.Render(h.Name), renderStatusPill(string(h.Status)))
		if h.CurrentFocus != "" {
			writeStrings(&rb, styleMuted.Render("Current focus: "), h.CurrentFocus, "\n")
		}
		if tags, _ := m.app.Store.ListTagsFor(m.ctx, domain.EntityHobby, h.ID); len(tags) > 0 {
			writeStrings(&rb, renderTagsLine(tags), "\n")
		}
		if h.Description != "" {
			writeStrings(&rb, "\n", h.Description, "\n")
		}

		logs, _ := m.app.Store.ListProgressLogs(m.ctx, domain.EntityHobby, h.ID)
		writeStrings(&rb, "\n", styleSectionHeader.Render(fmt.Sprintf("Progress & Reflection Log (%d) — [l] add entry", len(logs))), "\n")
		if len(logs) == 0 {
			writeStrings(&rb, "  ", styleMuted.Render("(no entries yet)"), "\n")
		} else {
			for i, lg := range logs {
				if i >= 6 {
					break
				}
				fmt.Fprintf(&rb, "  %s %s\n", styleMuted.Render(lg.LoggedAt.Format("2006-01-02")), truncate(lg.Content, rightW-16))
			}
		}
		rb.WriteString(m.renderLinkedSummary(domain.EntityHobby, h.ID, rightW))
		writeStrings(&rb, "\n", styleMuted.Render("[l] Add Progress Log   [c] Link Note/Resource   [g] Tag   [e] $EDITOR"))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, stylePaneFocused.Width(leftW).Render(lb.String()), " ", stylePane.Width(rightW).Render(rb.String()))
}

// --- 5. Routines ---

func (m *Model) updateRoutines(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.routineIdx < len(m.routines)-1 {
			m.routineIdx++
		}
	case "k", "up":
		if m.routineIdx > 0 {
			m.routineIdx--
		}
	case " ", "enter":
		if m.routineIdx < len(m.routines) {
			m.toggleRoutineCycle(m.routines[m.routineIdx])
		}
	case "n":
		m.openNewRoutineModal()
	case "a":
		if m.routineIdx < len(m.routines) {
			r := m.routines[m.routineIdx]
			r.Active = !r.Active
			if err := m.app.Store.UpdateRoutine(m.ctx, &r); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Routine %q active=%v", r.Title, r.Active))
				m.reload()
			}
		}
	case "m":
		if m.routineIdx < len(m.routines) {
			r := m.routines[m.routineIdx]
			r.ReminderEnabled = !r.ReminderEnabled
			if err := m.app.Store.UpdateRoutine(m.ctx, &r); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Routine %q reminder=%v", r.Title, r.ReminderEnabled))
				m.reload()
			}
		}
	case "d":
		if m.routineIdx < len(m.routines) {
			r := m.routines[m.routineIdx]
			m.modal = newConfirmModal("Delete Routine", fmt.Sprintf("Delete routine %q?", r.Title), func() error {
				if err := m.app.Store.DeleteRoutine(m.ctx, r.ID); err != nil {
					return err
				}
				m.setStatus("Deleted routine")
				return nil
			})
		}
	}
	return m, nil
}

func (m *Model) renderRoutines() string {
	now := m.clock()
	w := m.width - 4
	if w < 64 {
		w = 64
	}
	var b strings.Builder
	writeStrings(&b, styleSectionHeader.Render("Routines Checklist (Resets Automatically Each Cycle)"), "\n\n")
	if len(m.routines) == 0 {
		b.WriteString(styleMuted.Render("No routines configured yet. Press [n] to add daily/weekly/monthly/custom routines."))
	} else {
		for i, r := range m.routines {
			done, info, _ := domain.IsCompletedForCycle(r, now)
			check := "[ ]"
			if !r.Active {
				check = "[-]"
			} else if !info.IsDue {
				check = "[.]"
			} else if done {
				check = "[x]"
			}
			rem := ""
			if r.ReminderEnabled {
				if r.ReminderTime != "" {
					rem = " 🔔" + r.ReminderTime
				} else {
					rem = " 🔔"
				}
			}
			line := fmt.Sprintf("%s %-30s  %-18s%s", check, truncate(r.Title, 30), formatSchedule(r), rem)
			if i == m.routineIdx {
				writeStrings(&b, styleSelected.Render("▸ "+line), "\n")
			} else {
				writeStrings(&b, "  ", line, "\n")
			}
		}
	}
	return stylePaneFocused.Width(w).Render(b.String())
}

func (m *Model) toggleRoutineCycle(r domain.Routine) {
	now := m.clock()
	done, info, err := domain.IsCompletedForCycle(r, now)
	if err != nil {
		m.setErr(err)
		return
	}
	if !info.IsDue {
		m.setStatus(fmt.Sprintf("%q is not due today", r.Title))
		return
	}
	if done {
		if err := m.app.Store.ClearRoutineCompleted(m.ctx, r.ID); err != nil {
			m.setErr(err)
			return
		}
		m.setStatus(fmt.Sprintf("Unchecked %q", r.Title))
	} else {
		if err := m.app.Store.SetRoutineCompleted(m.ctx, r.ID, info.CycleKey, now.UTC()); err != nil {
			m.setErr(err)
			return
		}
		m.publish(domain.EventRoutineCompleted, domain.EntityRoutine, r.ID)
		m.setStatus(fmt.Sprintf("Completed %q for cycle %s", r.Title, info.CycleKey))
	}
	m.reload()
}

// --- 6. Notes & Links (Standalone Reusable Knowledge) ---

func (m *Model) updateKnowledge(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "v", "left", "right":
		m.knowledgeMode = 1 - m.knowledgeMode
		return m, nil
	}

	if m.knowledgeMode == 0 {
		// Notes sub-view.
		switch msg.String() {
		case "j", "down":
			if m.noteIdx < len(m.notes)-1 {
				m.noteIdx++
			}
		case "k", "up":
			if m.noteIdx > 0 {
				m.noteIdx--
			}
		case "n":
			m.openNewNoteModal()
		case "e", "enter":
			if m.noteIdx < len(m.notes) {
				nCopy := m.notes[m.noteIdx]
				return m, m.openExternalEditor(nCopy.Content, func(updated string) error {
					nCopy.Content = updated
					return m.app.Store.UpdateNote(m.ctx, &nCopy)
				})
			}
		case "g":
			if m.noteIdx < len(m.notes) {
				n := m.notes[m.noteIdx]
				m.openAddTagModal(domain.EntityNote, n.ID, n.Title)
			}
		case "c", "K":
			if m.noteIdx < len(m.notes) {
				n := m.notes[m.noteIdx]
				m.openLinkPickerModal(domain.EntityNote, n.ID, n.Title)
			}
		case "d":
			if m.noteIdx < len(m.notes) {
				n := m.notes[m.noteIdx]
				m.modal = newConfirmModal("Delete Note", fmt.Sprintf("Delete note %q?", n.Title), func() error {
					if err := m.app.Store.DeleteNote(m.ctx, n.ID); err != nil {
						return err
					}
					m.setStatus("Deleted note")
					return nil
				})
			}
		}
		return m, nil
	}

	// Resources / Links sub-view.
	switch msg.String() {
	case "j", "down":
		if m.resIdx < len(m.resources)-1 {
			m.resIdx++
		}
	case "k", "up":
		if m.resIdx > 0 {
			m.resIdx--
		}
	case "n":
		m.openNewResourceModal()
	case "s":
		if m.resIdx < len(m.resources) {
			r := m.resources[m.resIdx]
			r.Status = nextResourceStatus(r.Status)
			if err := m.app.Store.UpdateResource(m.ctx, &r); err != nil {
				m.setErr(err)
			} else {
				m.setStatus(fmt.Sprintf("Resource status → %s", r.Status))
				m.reload()
			}
		}
	case "g":
		if m.resIdx < len(m.resources) {
			r := m.resources[m.resIdx]
			m.openAddTagModal(domain.EntityResource, r.ID, r.Title)
		}
	case "c", "K":
		if m.resIdx < len(m.resources) {
			r := m.resources[m.resIdx]
			m.openLinkPickerModal(domain.EntityResource, r.ID, r.Title)
		}
	case "d":
		if m.resIdx < len(m.resources) {
			r := m.resources[m.resIdx]
			m.modal = newConfirmModal("Delete Resource", fmt.Sprintf("Delete resource %q?", r.Title), func() error {
				if err := m.app.Store.DeleteResource(m.ctx, r.ID); err != nil {
					return err
				}
				m.setStatus("Deleted resource")
				return nil
			})
		}
	}
	return m, nil
}

func (m *Model) renderKnowledge() string {
	leftW, rightW := m.paneWidths()

	var lb, rb strings.Builder
	if m.knowledgeMode == 0 {
		writeStrings(&lb, styleSectionHeader.Render("Notes (press [v] for Links/Resources)"), "\n\n")
		if len(m.notes) == 0 {
			lb.WriteString(styleMuted.Render("No notes yet. Press [n] to create one."))
		} else {
			for i, n := range m.notes {
				line := truncate(n.Title, leftW-6)
				if i == m.noteIdx {
					writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
				} else {
					writeStrings(&lb, "  ", line, "\n")
				}
			}
		}

		if m.noteIdx < len(m.notes) {
			n := m.notes[m.noteIdx]
			writeStrings(&rb, styleTitle.Render(n.Title), "\n")
			if tags, _ := m.app.Store.ListTagsFor(m.ctx, domain.EntityNote, n.ID); len(tags) > 0 {
				writeStrings(&rb, renderTagsLine(tags), "\n")
			}
			rb.WriteString("\n")
			if n.Content != "" {
				writeStrings(&rb, n.Content, "\n")
			} else {
				writeStrings(&rb, styleMuted.Render("(empty note — press [e] or [Enter] to edit in external $EDITOR)"), "\n")
			}
			rb.WriteString(m.renderLinkedSummary(domain.EntityNote, n.ID, rightW))
			writeStrings(&rb, "\n", styleMuted.Render("[e/Enter] Edit in $EDITOR   [c] Link to Entity   [g] Tag"))
		}
	} else {
		writeStrings(&lb, styleSectionHeader.Render("Resources & Links (press [v] for Notes)"), "\n\n")
		if len(m.resources) == 0 {
			lb.WriteString(styleMuted.Render("No links/resources yet. Press [n] to add one."))
		} else {
			for i, r := range m.resources {
				line := fmt.Sprintf("%s %s", truncate(r.Title, leftW-18), renderStatusPill(string(r.Status)))
				if i == m.resIdx {
					writeStrings(&lb, styleSelected.Render("▸ "+line), "\n")
				} else {
					writeStrings(&lb, "  ", line, "\n")
				}
			}
		}

		if m.resIdx < len(m.resources) {
			r := m.resources[m.resIdx]
			fmt.Fprintf(&rb, "%s  %s  (%s)\n", styleTitle.Render(r.Title), renderStatusPill(string(r.Status)), r.ResourceType)
			writeStrings(&rb, styleTag.Render(r.URLOrPath), "\n")
			if tags, _ := m.app.Store.ListTagsFor(m.ctx, domain.EntityResource, r.ID); len(tags) > 0 {
				writeStrings(&rb, renderTagsLine(tags), "\n")
			}
			if r.Notes != "" {
				writeStrings(&rb, "\n", r.Notes, "\n")
			}
			rb.WriteString(m.renderLinkedSummary(domain.EntityResource, r.ID, rightW))
			writeStrings(&rb, "\n", styleMuted.Render("[s] Cycle Status   [c] Link to Entity   [g] Tag"))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, stylePaneFocused.Width(leftW).Render(lb.String()), " ", stylePane.Width(rightW).Render(rb.String()))
}

// --- Modal Openers ---

func (m *Model) openNewProjectModal() {
	m.modal = newFormModal("New Project", "Create a scoped project with a clear desired outcome", [][3]string{
		{"Title", "e.g. Build Home NAS", ""},
		{"Goal / Desired Outcome", "What does success look like?", ""},
		{"Priority (low/medium/high)", "medium", "medium"},
	}, func(vals []string) error {
		prio := domain.Priority(strings.ToLower(vals[2]))
		if prio == "" {
			prio = domain.PriorityMedium
		}
		now := m.clock().UTC()
		p := &domain.Project{Title: vals[0], Goal: vals[1], Status: domain.ProjectActive, Priority: prio, StartDate: &now}
		if err := m.app.Store.CreateProject(m.ctx, p); err != nil {
			return err
		}
		m.publish(domain.EventProjectCreated, domain.EntityProject, p.ID)
		m.setStatus("Created project " + p.Title)
		return nil
	})
}

func (m *Model) openNewTaskModal(projectID, projectTitle string) {
	m.modal = newFormModal("New Task for "+projectTitle, "Add a concrete next action", [][3]string{
		{"Task Title", "e.g. Benchmark power draw", ""},
		{"Priority (low/medium/high)", "medium", "medium"},
		{"Due Date (optional YYYY-MM-DD)", "YYYY-MM-DD", ""},
	}, func(vals []string) error {
		prio := domain.Priority(strings.ToLower(vals[1]))
		if prio == "" {
			prio = domain.PriorityMedium
		}
		t := &domain.Task{ProjectID: projectID, Title: vals[0], Status: domain.TaskOpen, Priority: prio}
		if vals[2] != "" {
			parsed, err := time.ParseInLocation("2006-01-02", vals[2], time.Local)
			if err != nil {
				return fmt.Errorf("due date must be YYYY-MM-DD")
			}
			due := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 18, 0, 0, 0, time.Local).UTC()
			t.DueAt = &due
		}
		if err := m.app.Store.CreateTask(m.ctx, t); err != nil {
			return err
		}
		m.publish(domain.EventTaskCreated, domain.EntityTask, t.ID)
		m.setStatus("Added task " + t.Title)
		return nil
	})
}

func (m *Model) openNewIdeaModal() {
	m.modal = newFormModal("Capture Idea (Inbox)", "Get it out of your head — enrich with notes & links later", [][3]string{
		{"Title", "e.g. DIY Split Keyboard", ""},
		{"Source (optional)", "article, walk, friend...", ""},
		{"Excitement 1-5 (optional)", "4", ""},
	}, func(vals []string) error {
		idea := &domain.Idea{Title: vals[0], Source: vals[1], Status: domain.IdeaInbox}
		if vals[2] != "" {
			v, err := strconv.Atoi(vals[2])
			if err != nil {
				return fmt.Errorf("excitement must be 1..5")
			}
			idea.Excitement = &v
		}
		if err := m.app.Store.CreateIdea(m.ctx, idea); err != nil {
			return err
		}
		m.publish(domain.EventIdeaCreated, domain.EntityIdea, idea.ID)
		m.setStatus("Captured idea " + idea.Title)
		return nil
	})
}

func (m *Model) openPromoteIdeaModal(idea domain.Idea) {
	m.modal = newFormModal("Promote Idea → Project", "Carries over all linked notes, resources, and tags atomically", [][3]string{
		{"Project Title", idea.Title, idea.Title},
		{"Desired Outcome / Goal", "What does done look like?", ""},
		{"Priority (low/medium/high)", "medium", "medium"},
		{"Initial Next Actions (comma-separated)", "Research parts, Draft plan", ""},
	}, func(vals []string) error {
		prio := domain.Priority(strings.ToLower(vals[2]))
		if prio == "" {
			prio = domain.PriorityMedium
		}
		var tasks []string
		for _, tok := range strings.Split(vals[3], ",") {
			if s := strings.TrimSpace(tok); s != "" {
				tasks = append(tasks, s)
			}
		}
		proj, err := m.app.Store.PromoteIdeaToProject(m.ctx, domain.PromoteIdeaInput{
			IdeaID:       idea.ID,
			ProjectTitle: vals[0],
			Goal:         vals[1],
			Priority:     prio,
			InitialTasks: tasks,
		})
		if err != nil {
			return err
		}
		m.publish(domain.EventIdeaPromoted, domain.EntityProject, proj.ID)
		m.setStatus(fmt.Sprintf("Promoted %q to Project!", proj.Title))
		return nil
	})
}

func (m *Model) openNewHobbyModal() {
	m.modal = newFormModal("New Hobby / Ongoing Interest", "Ongoing domain with no fixed finish line", [][3]string{
		{"Name", "e.g. Woodworking", ""},
		{"Current Focus (optional)", "e.g. Hand-cut dovetails", ""},
	}, func(vals []string) error {
		h := &domain.Hobby{Name: vals[0], CurrentFocus: vals[1], Status: domain.HobbyActive}
		if err := m.app.Store.CreateHobby(m.ctx, h); err != nil {
			return err
		}
		m.publish(domain.EventHobbyCreated, domain.EntityHobby, h.ID)
		m.setStatus("Added hobby " + h.Name)
		return nil
	})
}

func (m *Model) openNewRoutineModal() {
	m.modal = newFormModal("New Routine", "Schedule: daily | weekly | monthly | every_n_days | weekdays", [][3]string{
		{"Title", "e.g. Morning Walk", ""},
		{"Schedule Type", "daily", "daily"},
		{"Custom Rule (N days or mon,wed,fri)", "leave blank unless custom", ""},
		{"Reminder HH:MM (optional opt-in)", "e.g. 08:00", ""},
	}, func(vals []string) error {
		st := domain.ScheduleType(strings.ToLower(vals[1]))
		var cfg domain.RoutineScheduleConfig
		switch st {
		case domain.ScheduleEveryNDays:
			n, err := strconv.Atoi(vals[2])
			if err != nil || n < 1 {
				return fmt.Errorf("enter interval days >= 1 for every_n_days")
			}
			cfg.IntervalDays = n
			cfg.AnchorDate = m.clock().Format("2006-01-02")
		case domain.ScheduleWeekdays:
			wds, err := parseTUIWeekdays(vals[2])
			if err != nil {
				return err
			}
			cfg.Weekdays = wds
		}
		r := &domain.Routine{
			Title:           vals[0],
			ScheduleType:    st,
			ScheduleConfig:  cfg,
			Active:          true,
			ReminderEnabled: vals[3] != "",
			ReminderTime:    vals[3],
		}
		if err := m.app.Store.CreateRoutine(m.ctx, r); err != nil {
			return err
		}
		m.publish(domain.EventRoutineCreated, domain.EntityRoutine, r.ID)
		m.setStatus("Added routine " + r.Title)
		return nil
	})
}

func (m *Model) openNewNoteModal() {
	m.modal = newFormModal("New Standalone Note", "After creating, press [e] to edit full Markdown in external $EDITOR", [][3]string{
		{"Title", "e.g. ZMK Firmware Notes", ""},
		{"Source (optional)", "book, article, conversation", ""},
	}, func(vals []string) error {
		n := &domain.Note{Title: vals[0], Source: vals[1]}
		if err := m.app.Store.CreateNote(m.ctx, n); err != nil {
			return err
		}
		m.publish(domain.EventNoteCreated, domain.EntityNote, n.ID)
		m.setStatus("Created note " + n.Title)
		return nil
	})
}

func (m *Model) openNewResourceModal() {
	m.modal = newFormModal("New Resource / Link", "Type: link | book | course | tool | file | other", [][3]string{
		{"Title", "e.g. Corne Keyboard Repo", ""},
		{"URL or Path", "https://...", ""},
		{"Type", "link", "link"},
	}, func(vals []string) error {
		rt := domain.ResourceType(strings.ToLower(vals[2]))
		if rt == "" {
			rt = domain.ResourceLink
		}
		r := &domain.Resource{Title: vals[0], URLOrPath: vals[1], ResourceType: rt, Status: domain.ResourceReference}
		if err := m.app.Store.CreateResource(m.ctx, r); err != nil {
			return err
		}
		m.publish(domain.EventResourceCreated, domain.EntityResource, r.ID)
		m.setStatus("Added resource " + r.Title)
		return nil
	})
}

func (m *Model) openAddLogModal(entType domain.EntityType, entID, title string) {
	m.modal = newFormModal("Log Progress — "+title, "Dated progress or reflection entry", [][3]string{
		{"What happened / learned / changed?", "Completed prototype wiring...", ""},
	}, func(vals []string) error {
		entry := &domain.ProgressLog{
			EntityType: entType,
			EntityID:   entID,
			LoggedAt:   m.clock().UTC(),
			Content:    vals[0],
		}
		if err := m.app.Store.AddProgressLog(m.ctx, entry); err != nil {
			return err
		}
		m.publish(domain.EventProgressLogged, entType, entID)
		m.setStatus("Added progress log")
		return nil
	})
}

func (m *Model) openAddTagModal(entType domain.EntityType, entID, title string) {
	m.modal = newFormModal("Tag — "+title, "Enter comma-separated tags (e.g. tech, home, energy:high)", [][3]string{
		{"Tags", "golang, diy", ""},
	}, func(vals []string) error {
		for _, tok := range strings.Split(vals[0], ",") {
			if t := strings.TrimSpace(tok); t != "" {
				if _, err := m.app.Store.TagEntity(m.ctx, entType, entID, t); err != nil {
					return err
				}
			}
		}
		m.setStatus("Updated tags")
		return nil
	})
}

func (m *Model) openLinkPickerModal(srcType domain.EntityType, srcID, title string) {
	var opts []pickerOption
	for _, n := range m.notes {
		if srcType == domain.EntityNote && srcID == n.ID {
			continue
		}
		opts = append(opts, pickerOption{Label: n.Title, EntityType: domain.EntityNote, EntityID: n.ID})
	}
	for _, r := range m.resources {
		if srcType == domain.EntityResource && srcID == r.ID {
			continue
		}
		opts = append(opts, pickerOption{Label: r.Title + " (" + truncate(r.URLOrPath, 28) + ")", EntityType: domain.EntityResource, EntityID: r.ID})
	}
	for _, h := range m.hobbies {
		if srcType == domain.EntityHobby && srcID == h.ID {
			continue
		}
		opts = append(opts, pickerOption{Label: h.Name, EntityType: domain.EntityHobby, EntityID: h.ID})
	}
	for _, p := range m.projects {
		if srcType == domain.EntityProject && srcID == p.ID {
			continue
		}
		opts = append(opts, pickerOption{Label: p.Title, EntityType: domain.EntityProject, EntityID: p.ID})
	}
	for _, it := range m.ideas {
		if srcType == domain.EntityIdea && srcID == it.ID {
			continue
		}
		opts = append(opts, pickerOption{Label: it.Title, EntityType: domain.EntityIdea, EntityID: it.ID})
	}

	m.modal = newPickerModal("Link to "+title, "Choose a Note, Resource, Hobby, Project, or Idea", opts, func(opt pickerOption) error {
		if err := m.app.Store.AddLink(m.ctx, domain.EntityLink{
			SourceType:   srcType,
			SourceID:     srcID,
			TargetType:   opt.EntityType,
			TargetID:     opt.EntityID,
			RelationType: "related",
		}); err != nil {
			return err
		}
		m.setStatus(fmt.Sprintf("Linked [%s] %s", opt.EntityType, opt.Label))
		return nil
	})
}

// --- Helpers ---

func (m *Model) renderLinkedSummary(entType domain.EntityType, entID string, maxW int) string {
	links, err := m.app.Store.ListLinksFor(m.ctx, entType, entID)
	if err != nil || len(links) == 0 {
		return ""
	}
	var b strings.Builder
	writeStrings(&b, "\n", styleSectionHeader.Render(fmt.Sprintf("Linked Items (%d)", len(links))), "\n")
	for i, lk := range links {
		if i >= 6 {
			writeStrings(&b, "  ", styleMuted.Render(fmt.Sprintf("+%d more links", len(links)-6)), "\n")
			break
		}
		otherType, otherID := lk.TargetType, lk.TargetID
		if lk.TargetType == entType && lk.TargetID == entID {
			otherType, otherID = lk.SourceType, lk.SourceID
		}
		label := m.resolveEntityLabel(otherType, otherID)
		fmt.Fprintf(&b, "  • [%s] %s %s\n", otherType, truncate(label, maxW-24), styleMuted.Render("("+lk.RelationType+")"))
	}
	return b.String()
}

func (m *Model) resolveEntityLabel(et domain.EntityType, id string) string {
	switch et {
	case domain.EntityProject:
		for _, p := range m.projects {
			if p.ID == id {
				return p.Title
			}
		}
	case domain.EntityIdea:
		for _, it := range m.ideas {
			if it.ID == id {
				return it.Title
			}
		}
	case domain.EntityHobby:
		for _, h := range m.hobbies {
			if h.ID == id {
				return h.Name
			}
		}
	case domain.EntityNote:
		for _, n := range m.notes {
			if n.ID == id {
				return n.Title
			}
		}
	case domain.EntityResource:
		for _, r := range m.resources {
			if r.ID == id {
				return r.Title
			}
		}
	}
	if len(id) > 8 {
		return id[len(id)-8:]
	}
	return id
}

func renderTagsLine(tags []domain.Tag) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = styleTag.Render("#" + t.Name)
	}
	return strings.Join(parts, " ")
}

func formatSchedule(r domain.Routine) string {
	switch r.ScheduleType {
	case domain.ScheduleEveryNDays:
		return fmt.Sprintf("every %dd", r.ScheduleConfig.IntervalDays)
	case domain.ScheduleWeekdays:
		names := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		var parts []string
		for _, wd := range r.ScheduleConfig.Weekdays {
			if wd >= 0 && wd <= 6 {
				parts = append(parts, names[wd])
			}
		}
		return strings.Join(parts, ",")
	default:
		return string(r.ScheduleType)
	}
}

func parseTUIWeekdays(csv string) ([]int, error) {
	m := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	var out []int
	for _, tok := range strings.Split(csv, ",") {
		s := strings.ToLower(strings.TrimSpace(tok))
		if s == "" {
			continue
		}
		if v, ok := m[s]; ok {
			out = append(out, v)
		} else if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 6 {
			out = append(out, n)
		} else {
			return nil, fmt.Errorf("invalid weekday %q (use mon,wed,fri or 0..6)", tok)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("provide at least one weekday (e.g. mon,wed,fri)")
	}
	return out, nil
}

func projectFilterLabel(f int) string {
	switch f {
	case 0:
		return "Active & Waiting"
	case 1:
		return "All"
	case 2:
		return "On Hold"
	default:
		return "Finished & Rejected"
	}
}

func ideaFilterLabel(f int) string {
	switch f {
	case 0:
		return "Active Inbox"
	case 1:
		return "Parked"
	default:
		return "All"
	}
}

func nextProjectStatus(s domain.ProjectStatus) domain.ProjectStatus {
	order := []domain.ProjectStatus{domain.ProjectActive, domain.ProjectWaiting, domain.ProjectOnHold, domain.ProjectFinished, domain.ProjectRejected}
	for i, v := range order {
		if v == s {
			return order[(i+1)%len(order)]
		}
	}
	return domain.ProjectActive
}

func nextTaskStatus(s domain.TaskStatus) domain.TaskStatus {
	order := []domain.TaskStatus{domain.TaskOpen, domain.TaskInProgress, domain.TaskBlocked, domain.TaskDone, domain.TaskCancelled}
	for i, v := range order {
		if v == s {
			return order[(i+1)%len(order)]
		}
	}
	return domain.TaskOpen
}

func nextPriority(p domain.Priority) domain.Priority {
	switch p {
	case domain.PriorityLow:
		return domain.PriorityMedium
	case domain.PriorityMedium:
		return domain.PriorityHigh
	default:
		return domain.PriorityLow
	}
}

func nextIdeaStatus(s domain.IdeaStatus) domain.IdeaStatus {
	order := []domain.IdeaStatus{domain.IdeaInbox, domain.IdeaExploring, domain.IdeaReady, domain.IdeaParked, domain.IdeaDiscarded}
	for i, v := range order {
		if v == s {
			return order[(i+1)%len(order)]
		}
	}
	return domain.IdeaInbox
}

func nextHobbyStatus(s domain.HobbyStatus) domain.HobbyStatus {
	switch s {
	case domain.HobbyActive:
		return domain.HobbyOnHold
	case domain.HobbyOnHold:
		return domain.HobbyRejected
	default:
		return domain.HobbyActive
	}
}

func nextResourceStatus(s domain.ResourceStatus) domain.ResourceStatus {
	switch s {
	case domain.ResourceToConsume:
		return domain.ResourceInProgress
	case domain.ResourceInProgress:
		return domain.ResourceReference
	default:
		return domain.ResourceToConsume
	}
}
