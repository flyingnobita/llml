package tui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func (m Model) openImportView() (Model, tea.Cmd) {
	m.import_.open = true
	m.import_.focus = importFocusPicker
	m.import_.filePath = ""
	m.import_.pathInput.SetValue("")
	m.import_.pathInput.Blur()
	m.import_.groups = nil
	m.import_.cursor = 0
	m.import_.scrollOffset = 0
	m.import_.parseError = ""
	m.import_.pathInput.SetWidth(PathTextInputWidth)
	m.import_.picker = filepicker.New()
	m.import_.picker.AllowedTypes = []string{".toml"}
	m.import_.picker.DirAllowed = true
	m.import_.picker.FileAllowed = true
	m.import_.picker.AutoHeight = true
	m.import_.picker.ShowHidden = false
	if m.layout.homeDir != "" {
		m.import_.picker.CurrentDirectory = m.layout.homeDir
	}
	m.import_.picker.SetHeight(m.importPickerBodyH())
	m = m.clearCurrentStatus()
	return m, m.import_.picker.Init()
}

func (m Model) closeImportView() Model {
	m.import_.open = false
	m.import_.pathInput.Blur()
	m.import_.groups = nil
	m.import_.parseError = ""
	return m
}

func (m Model) parseImportFile() Model {
	m.import_.parseError = ""
	m.import_.groups = nil
	m.import_.cursor = 0
	m.import_.scrollOffset = 0

	path := strings.TrimSpace(m.import_.filePath)
	if path == "" {
		m.import_.parseError = "Enter a file path."
		return m
	}

	f, err := profiles.ReadPortable(path)
	if err != nil {
		m.import_.parseError = err.Error()
		return m
	}

	type hintGroup struct {
		modelHint string
		profiles  []profiles.PortableProfile
	}
	var ordered []hintGroup
	hintIndex := make(map[string]int)
	for _, pp := range f.Profiles {
		hint := strings.TrimSpace(pp.ModelHint)
		if hint == "" {
			hint = "(no hint)"
		}
		if idx, ok := hintIndex[hint]; ok {
			ordered[idx].profiles = append(ordered[idx].profiles, pp)
		} else {
			hintIndex[hint] = len(ordered)
			ordered = append(ordered, hintGroup{modelHint: hint, profiles: []profiles.PortableProfile{pp}})
		}
	}

	for _, hg := range ordered {
		g := importGroup{
			modelHint: hg.modelHint,
			profiles:  hg.profiles,
			checked:   true,
		}
		if matched, ok := fuzzyMatchModelHint(hg.modelHint, m.table.files); ok {
			g.matchedKey = matched.Identity()
			g.matchedDisplay = matched.DisplayLocation()
			if g.matchedDisplay == "" {
				g.matchedDisplay = matched.Name
			}
		}
		m.import_.groups = append(m.import_.groups, g)
	}

	m.import_.focus = importFocusList
	return m
}

// fuzzyMatchModelHint matches a portable model_hint against the scanned model list.
func fuzzyMatchModelHint(hint string, files []models.ModelFile) (models.ModelFile, bool) {
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", "")
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}
	h := norm(hint)
	for _, sfx := range []string{"gguf", "safetensors", "ggml"} {
		h = strings.TrimSuffix(h, sfx)
	}
	if h == "" {
		return models.ModelFile{}, false
	}
	for _, f := range files {
		if strings.Contains(norm(f.Name), h) || strings.Contains(norm(f.Path), h) {
			return f, true
		}
	}
	return models.ModelFile{}, false
}

func (m Model) updateImportKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.import_.focus {
	case importFocusPath:
		switch {
		case isEscapeKey(msg):
			return m.closeImportView(), nil
		case msg.String() == "tab":
			m.import_.focus = importFocusPicker
			m.import_.picker.Path = ""
			m.import_.picker.SetHeight(m.importPickerBodyH())
			return m, m.import_.picker.Init()
		case msg.String() == "enter":
			m = m.parseImportFile()
			return m, nil
		default:
			var cmd tea.Cmd
			m.import_.pathInput, cmd = m.import_.pathInput.Update(msg)
			m.import_.filePath = m.import_.pathInput.Value()
			return m, cmd
		}
	case importFocusPicker:
		switch {
		case isEscapeKey(msg):
			return m.closeImportView(), nil
		case msg.String() == "tab":
			m.import_.focus = importFocusPath
			m.import_.pathInput.Focus()
			return m, nil
		default:
			var cmd tea.Cmd
			m.import_.picker, cmd = m.import_.picker.Update(msg)
			if m.import_.picker.Path != "" && !isDir(m.import_.picker.Path) {
				m.import_.filePath = m.import_.picker.Path
				m.import_.pathInput.SetValue(m.import_.picker.Path)
				m.import_.focus = importFocusPath
				m.import_.pathInput.Focus()
				return m.parseImportFile(), cmd
			}
			return m, cmd
		}
	case importFocusList:
		switch {
		case isEscapeKey(msg):
			m.import_.focus = importFocusPath
			m.import_.pathInput.Focus()
			m.import_.parseError = ""
			return m, nil
		case msg.String() == " ":
			if len(m.import_.groups) > 0 && m.import_.cursor < len(m.import_.groups) {
				m.import_.groups[m.import_.cursor].checked = !m.import_.groups[m.import_.cursor].checked
			}
			return m, nil
		case msg.String() == "enter":
			return m.doImportAttempt()
		case msg.String() == "j", msg.String() == "down":
			if m.import_.cursor < len(m.import_.groups)-1 {
				m.import_.cursor++
			}
			return m, nil
		case msg.String() == "k", msg.String() == "up":
			if m.import_.cursor > 0 {
				m.import_.cursor--
			}
			return m, nil
		case msg.String() == "ctrl+u":
			m.import_.cursor = max(m.import_.cursor-5, 0)
			return m, nil
		case msg.String() == "ctrl+d":
			m.import_.cursor = min(m.import_.cursor+5, len(m.import_.groups)-1)
			return m, nil
		case msg.String() == "a":
			for i := range m.import_.groups {
				m.import_.groups[i].checked = true
			}
			return m, nil
		case msg.String() == "A":
			for i := range m.import_.groups {
				m.import_.groups[i].checked = false
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) doImportAttempt() (tea.Model, tea.Cmd) {
	totalAdded, totalReplaced, totalSkipped := 0, 0, 0
	var importedModels []string

	for _, g := range m.import_.groups {
		if !g.checked || g.matchedKey == "" {
			if g.checked && g.matchedKey == "" {
				totalSkipped += len(g.profiles)
			}
			continue
		}
		var ps []profiles.Profile
		for _, pp := range g.profiles {
			p := profiles.PortableToProfile(pp)
			if _, _, droppedEnv, droppedArgs := profiles.StripModelLocationParams(
				p.Backend, pp.Env, pp.Args); len(droppedEnv) > 0 || len(droppedArgs) > 0 {
				var parts []string
				for _, d := range droppedEnv {
					parts = append(parts, "env:"+d)
				}
				for _, d := range droppedArgs {
					parts = append(parts, "arg:"+d)
				}
				m = m.addAlert(alertSeverityWarn, "Import",
					fmt.Sprintf("Stripped model-location params from %q: %s", p.Name, strings.Join(parts, ", ")))
			}
			ps = append(ps, p)
		}
		result, err := profiles.ImportProfiles(g.matchedKey, ps, false)
		if err != nil {
			return m.flashError(fmt.Sprintf("Import failed for %s: %v", g.matchedKey, err))
		}
		totalAdded += result.Added
		totalReplaced += result.Replaced
		totalSkipped += result.Skipped
		if result.Added > 0 || result.Replaced > 0 {
			importedModels = append(importedModels, g.matchedKey)
		}
	}

	summary := fmt.Sprintf("Imported %d profiles across %d models", totalAdded+totalReplaced, len(importedModels))
	if totalSkipped > 0 {
		summary += fmt.Sprintf(" (%d skipped)", totalSkipped)
	}
	m = m.addAlert(alertSeverityInfo, "Import", summary)
	m2, cmd := m.flashSuccess(summary)
	m = m2
	m = m.closeImportView()
	return m, cmd
}

func (m Model) importModalBlock() string {
	st := m.ui.styles

	if m.import_.focus == importFocusPicker {
		return m.importPickerModalBlock()
	}

	title := st.portConfigTitle.Render("Import Profiles")
	footer := m.importFooter()

	body := m.importBodyView()

	block := st.paramPanelBox.Render(title + "\n" + body + "\n" + footer)
	return block
}

// importPickerBodyH returns the available height for the filepicker file list,
// accounting for the modal frame (title, footer, border).
func (m Model) importPickerBodyH() int {
	th := m.layout.height
	if th < 10 {
		th = 10
	}
	h := th * 85 / 100
	if h < 8 {
		h = 8
	}
	title := m.ui.styles.portConfigTitle.Render(" Browse TOML File ")
	footer := m.importFooter()
	bodyH := h - lipgloss.Height(title) - lipgloss.Height(footer) - 2
	if bodyH < 4 {
		bodyH = 4
	}
	return bodyH
}

func (m Model) importPickerModalBlock() string {
	st := m.ui.styles

	tw := m.layout.width
	th := m.layout.height
	if tw < 40 {
		tw = 40
	}
	if th < 10 {
		th = 10
	}

	// Use 90% width, 85% height to leave some margin around the overlay.
	w := tw * 9 / 10
	h := th * 85 / 100
	if w < 40 {
		w = 40
	}
	if h < 8 {
		h = 8
	}

	title := st.portConfigTitle.Render(" Browse TOML File ")
	footer := m.importFooter()

	bodyH := m.importPickerBodyH()
	m.import_.picker.SetHeight(bodyH)

	pickerView := m.import_.picker.View()

	content := title + "\n" + pickerView + "\n" + footer

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.ui.theme.Border).
		Padding(0, 2).
		Width(w).
		Height(h).
		Render(content)

	return box
}

func (m Model) importFooter() string {
	st := m.ui.styles
	switch m.import_.focus {
	case importFocusPath:
		return st.footer.Render("tab: browse · enter: parse · esc: back")
	case importFocusPicker:
		return st.footer.Render("tab: path input · enter: select · " + FooterNavHint + " · esc: back")
	default:
		return st.footer.Render("space: toggle · a: all · A: none · enter: import · esc: back")
	}
}

func (m Model) importBodyView() string {
	st := m.ui.styles
	var b strings.Builder

	if m.import_.focus == importFocusPicker {
		b.WriteString(m.import_.picker.View())
		return b.String()
	}

	// Path input
	pathLabel := st.bodyBold.Render("File:")
	pathVal := m.import_.pathInput.View()
	b.WriteString(pathLabel + " " + pathVal + "\n")

	// Parse error
	if m.import_.parseError != "" {
		b.WriteString(st.bodyDim.Render(m.import_.parseError) + "\n")
		return b.String()
	}

	if len(m.import_.groups) == 0 {
		b.WriteString(st.bodyDim.Render("No profiles to show.") + "\n")
		return b.String()
	}

	// Group list
	b.WriteString("\n")
	visibleStart, visibleEnd := m.importVisibleRange()
	for i := range m.import_.groups {
		if i < visibleStart || i >= visibleEnd {
			continue
		}
		g := m.import_.groups[i]
		b.WriteString(m.renderImportGroupRow(i, g))
	}

	return b.String()
}

func (m Model) renderImportGroupRow(i int, g importGroup) string {
	st := m.ui.styles
	cursor := "  "
	if i == m.import_.cursor && m.import_.focus == importFocusList {
		cursor = st.bodyBold.Render("> ")
	}

	checkbox := "[ ]"
	if g.checked {
		checkbox = "[" + st.bodyBold.Render("x") + "]"
	}

	hint := st.bodyBold.Render(g.modelHint)
	status := ""
	if g.matchedKey != "" {
		display := g.matchedDisplay
		if display == "" {
			display = g.matchedKey
		}
		status = " → " + st.body.Render(display)
	} else {
		status = " " + st.bodyDim.Render("[not found — will skip]")
	}

	count := fmt.Sprintf("(%d profile", len(g.profiles))
	if len(g.profiles) > 1 {
		count += "s"
	}
	count += ")"

	return cursor + checkbox + " " + hint + status + " " + st.bodyDim.Render(count) + "\n"
}

func (m Model) importVisibleRange() (start, end int) {
	total := len(m.import_.groups)
	if total == 0 {
		return 0, 0
	}
	maxVis := m.importMaxVisibleItems()
	if total <= maxVis {
		return 0, total
	}
	// Keep cursor in view
	if m.import_.cursor < m.import_.scrollOffset {
		m.import_.scrollOffset = m.import_.cursor
	}
	if m.import_.cursor >= m.import_.scrollOffset+maxVis {
		m.import_.scrollOffset = m.import_.cursor - maxVis + 1
	}
	if m.import_.scrollOffset < 0 {
		m.import_.scrollOffset = 0
	}
	if m.import_.scrollOffset > total-maxVis {
		m.import_.scrollOffset = total - maxVis
	}
	return m.import_.scrollOffset, m.import_.scrollOffset + maxVis
}

// importMaxVisibleItems returns how many profile groups can fit in the terminal.
func (m Model) importMaxVisibleItems() int {
	termH := m.layout.height
	if termH < 1 {
		termH = 24
	}
	// Reserve space for title, path, footer, borders (~15 lines)
	n := termH - 16
	if n < 4 {
		return 4
	}
	if n > 30 {
		return 30
	}
	return n
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
