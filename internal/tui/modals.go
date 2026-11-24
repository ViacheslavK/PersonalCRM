package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"personalcrm/internal/domain"
)

type modalKind int

const (
	modalNone modalKind = iota
	modalForm
	modalPicker
	modalConfirm
)

type formField struct {
	Label string
	Input textinput.Model
}

type pickerOption struct {
	Label      string
	EntityType domain.EntityType
	EntityID   string
}

type activeModal struct {
	Kind       modalKind
	Title      string
	Hint       string
	Fields     []formField
	FieldIdx   int
	OnSubmit   func(values []string) error
	Options    []pickerOption
	PickerIdx  int
	OnPick     func(opt pickerOption) error
	ConfirmMsg string
	OnConfirm  func() error
}

func newFormModal(title, hint string, specs [][3]string, onSubmit func([]string) error) *activeModal {
	fields := make([]formField, len(specs))
	for i, sp := range specs {
		ti := textinput.New()
		ti.Placeholder = sp[1]
		ti.SetValue(sp[2])
		ti.CharLimit = 256
		ti.Width = 46
		if i == 0 {
			ti.Focus()
		}
		fields[i] = formField{Label: sp[0], Input: ti}
	}
	return &activeModal{
		Kind:     modalForm,
		Title:    title,
		Hint:     hint,
		Fields:   fields,
		OnSubmit: onSubmit,
	}
}

func newPickerModal(title, hint string, opts []pickerOption, onPick func(pickerOption) error) *activeModal {
	return &activeModal{
		Kind:    modalPicker,
		Title:   title,
		Hint:    hint,
		Options: opts,
		OnPick:  onPick,
	}
}

func newConfirmModal(title, msg string, onConfirm func() error) *activeModal {
	return &activeModal{
		Kind:       modalConfirm,
		Title:      title,
		ConfirmMsg: msg,
		OnConfirm:  onConfirm,
	}
}

func (m *Model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	mod := m.modal
	if mod == nil {
		return m, nil
	}

	switch mod.Kind {
	case modalConfirm:
		switch msg.String() {
		case "y", "Y", "enter":
			cb := mod.OnConfirm
			m.modal = nil
			if cb != nil {
				if err := cb(); err != nil {
					m.setErr(err)
				} else {
					m.reload()
				}
			}
		case "n", "N", "esc":
			m.modal = nil
			m.setStatus("Cancelled")
		}
		return m, nil

	case modalPicker:
		switch msg.String() {
		case "esc", "q":
			m.modal = nil
		case "j", "down":
			if mod.PickerIdx < len(mod.Options)-1 {
				mod.PickerIdx++
			}
		case "k", "up":
			if mod.PickerIdx > 0 {
				mod.PickerIdx--
			}
		case "enter":
			if len(mod.Options) == 0 {
				m.modal = nil
				return m, nil
			}
			chosen := mod.Options[mod.PickerIdx]
			cb := mod.OnPick
			m.modal = nil
			if cb != nil {
				if err := cb(chosen); err != nil {
					m.setErr(err)
				} else {
					m.reload()
				}
			}
		}
		return m, nil

	case modalForm:
		switch msg.String() {
		case "esc":
			m.modal = nil
			return m, nil
		case "tab", "down":
			if len(mod.Fields) > 1 {
				mod.Fields[mod.FieldIdx].Input.Blur()
				mod.FieldIdx = (mod.FieldIdx + 1) % len(mod.Fields)
				mod.Fields[mod.FieldIdx].Input.Focus()
			}
			return m, nil
		case "shift+tab", "up":
			if len(mod.Fields) > 1 {
				mod.Fields[mod.FieldIdx].Input.Blur()
				mod.FieldIdx = (mod.FieldIdx - 1 + len(mod.Fields)) % len(mod.Fields)
				mod.Fields[mod.FieldIdx].Input.Focus()
			}
			return m, nil
		case "enter":
			if mod.FieldIdx < len(mod.Fields)-1 {
				mod.Fields[mod.FieldIdx].Input.Blur()
				mod.FieldIdx++
				mod.Fields[mod.FieldIdx].Input.Focus()
				return m, nil
			}
			vals := make([]string, len(mod.Fields))
			for i := range mod.Fields {
				vals[i] = strings.TrimSpace(mod.Fields[i].Input.Value())
			}
			cb := mod.OnSubmit
			m.modal = nil
			if cb != nil {
				if err := cb(vals); err != nil {
					m.setErr(err)
				} else {
					m.reload()
				}
			}
			return m, nil
		}

		var cmd tea.Cmd
		mod.Fields[mod.FieldIdx].Input, cmd = mod.Fields[mod.FieldIdx].Input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) renderModal() string {
	mod := m.modal
	if mod == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render(mod.Title) + "\n")
	if mod.Hint != "" {
		b.WriteString(styleMuted.Render(mod.Hint) + "\n")
	}
	b.WriteString("\n")

	switch mod.Kind {
	case modalConfirm:
		b.WriteString(mod.ConfirmMsg + "\n\n")
		b.WriteString(styleMuted.Render("[y/Enter] Confirm   [n/Esc] Cancel"))

	case modalPicker:
		if len(mod.Options) == 0 {
			b.WriteString(styleMuted.Render("No items available to link.") + "\n\n")
			b.WriteString(styleMuted.Render("[Esc] Close"))
		} else {
			start := 0
			maxVisible := 12
			if mod.PickerIdx >= maxVisible {
				start = mod.PickerIdx - maxVisible + 1
			}
			end := start + maxVisible
			if end > len(mod.Options) {
				end = len(mod.Options)
			}
			for i := start; i < end; i++ {
				opt := mod.Options[i]
				prefix := "  "
				line := fmt.Sprintf("[%s] %s", opt.EntityType, opt.Label)
				if i == mod.PickerIdx {
					prefix = "▸ "
					line = styleSelected.Render(line)
				}
				b.WriteString(prefix + line + "\n")
			}
			b.WriteString("\n" + styleMuted.Render("[↑/↓] Move   [Enter] Link   [Esc] Cancel"))
		}

	case modalForm:
		for i, f := range mod.Fields {
			label := f.Label
			if i == mod.FieldIdx {
				label = styleSelected.Render("▸ " + label)
			} else {
				label = "  " + styleMuted.Render(label)
			}
			b.WriteString(label + "\n")
			b.WriteString("  " + f.Input.View() + "\n\n")
		}
		b.WriteString(styleMuted.Render("[Tab/↑/↓] Next field   [Enter] Next/Submit   [Esc] Cancel"))
	}

	return styleModal.Render(b.String())
}
