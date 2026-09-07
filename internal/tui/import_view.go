package tui

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/flyingnobita/llml/internal/models"
	"github.com/flyingnobita/llml/internal/profiles"
)

func (m Model) openImportView() (Model, tea.Cmd) {
	m.importView.open = true
	m.importView.focus = importFocusPicker
	m.importView.filePath = ""
	m.importView.pathInput.SetValue("")
	m.importView.pathInput.Blur()
	m.importView.groups = nil
	m.importView.cursor = 0
	m.importView.scrollOffset = 0
	m.importView.parseError = ""
	m.importView.pathInput.SetWidth(PathTextInputWidth)
	m.importView.picker = filepicker.New()
	m.importView.picker.AllowedTypes = []string{".toml"}
	m.importView.picker.DirAllowed = true
	m.importView.picker.FileAllowed = true
	m.importView.picker.AutoHeight = true
	m.importView.picker.ShowHidden = false
	if m.layout.homeDir != "" {
		m.importView.picker.CurrentDirectory = m.layout.homeDir
	}
	m.importView.picker.SetHeight(m.importPickerBodyH())
	m = m.clearCurrentStatus()
	return m, m.importView.picker.Init()
}

func (m Model) closeImportView() Model {
	m.importView.open = false
	m.importView.pathInput.Blur()
	m.importView.groups = nil
	m.importView.parseError = ""
	return m
}

func (m Model) parseImportFile() Model {
	m.importView.parseError = ""
	m.importView.groups = nil
	m.importView.cursor = 0
	m.importView.scrollOffset = 0

	path := strings.TrimSpace(m.importView.filePath)
	if path == "" {
		m.importView.parseError = "Enter a file path."
		return m
	}

	f, err := profiles.ReadPortable(path)
	if err != nil {
		m.importView.parseError = err.Error()
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
		m.importView.groups = append(m.importView.groups, g)
	}

	m.importView.focus = importFocusList
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

// updateImportKey routes a key press to the import panel section that has focus.
func (m Model) updateImportKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.importView.focus {
	case importFocusPath:
		return m.updateImportPathKey(msg)
	case importFocusPicker:
		return m.updateImportPickerKey(msg)
	case importFocusList:
		return m.updateImportListKey(msg)
	}
	return m, nil
}

func (m Model) updateImportPathKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isEscapeKey(msg):
		return m.closeImportView(), nil
	case isTabKey(msg):
		m.importView.focus = importFocusPicker
		m.importView.picker.Path = ""
		m.importView.picker.SetHeight(m.importPickerBodyH())
		return m, m.importView.picker.Init()
	case isEnterKey(msg):
		return m.parseImportFile(), nil
	}
	var cmd tea.Cmd
	m.importView.pathInput, cmd = m.importView.pathInput.Update(msg)
	m.importView.filePath = m.importView.pathInput.Value()
	return m, cmd
}

func (m Model) updateImportPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isEscapeKey(msg):
		return m.closeImportView(), nil
	case isTabKey(msg):
		m.importView.focus = importFocusPath
		m.importView.pathInput.Focus()
		return m, nil
	}
	var cmd tea.Cmd
	m.importView.picker, cmd = m.importView.picker.Update(msg)
	// Picking a file (rather than descending into a directory) moves focus to
	// the path field and parses immediately.
	if m.importView.picker.Path != "" && !isDir(m.importView.picker.Path) {
		m.importView.filePath = m.importView.picker.Path
		m.importView.pathInput.SetValue(m.importView.picker.Path)
		m.importView.focus = importFocusPath
		m.importView.pathInput.Focus()
		return m.parseImportFile(), cmd
	}
	return m, cmd
}

func (m Model) updateImportListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isEscapeKey(msg):
		m.importView.focus = importFocusPath
		m.importView.pathInput.Focus()
		m.importView.parseError = ""
		return m, nil
	case isEnterKey(msg):
		return m.doImportAttempt()
	}

	last := len(m.importView.groups) - 1
	switch msg.String() {
	case " ":
		if m.importView.cursor >= 0 && m.importView.cursor <= last {
			m = m.withImportGroupsCloned()
			m.importView.groups[m.importView.cursor].checked = !m.importView.groups[m.importView.cursor].checked
		}
	case "j", "down":
		m.importView.cursor = clampIndex(m.importView.cursor+1, last)
	case "k", "up":
		m.importView.cursor = clampIndex(m.importView.cursor-1, last)
	case "ctrl+d":
		m.importView.cursor = clampIndex(m.importView.cursor+5, last)
	case "ctrl+u":
		m.importView.cursor = clampIndex(m.importView.cursor-5, last)
	case "a":
		m = m.setAllImportGroupsChecked(true)
	case "A":
		m = m.setAllImportGroupsChecked(false)
	}
	return m, nil
}

// setAllImportGroupsChecked checks or unchecks every profile group at once.
func (m Model) setAllImportGroupsChecked(checked bool) Model {
	m = m.withImportGroupsCloned()
	for i := range m.importView.groups {
		m.importView.groups[i].checked = checked
	}
	return m
}

func (m Model) doImportAttempt() (tea.Model, tea.Cmd) {
	totalAdded, totalReplaced, totalSkipped := 0, 0, 0
	var importedModels []string

	for _, g := range m.importView.groups {
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

	if m.importView.focus == importFocusPicker {
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
	m.importView.picker.SetHeight(bodyH)

	pickerView := m.importView.picker.View()

	content := title + "\n" + pickerView + "\n" + footer

	return m.ui.styles.importBox.Width(w).Height(h).Render(content)
}

func (m Model) importFooter() string {
	switch m.importView.focus {
	case importFocusPath:
		return m.renderFooterHints("tab: browse · enter: parse · esc: back")
	case importFocusPicker:
		return m.renderFooterHints("tab: path input · enter: select · " + FooterNavHint + " · esc: back")
	default:
		return m.renderFooterHints("space: toggle · a: all · A: none · enter: import · esc: back")
	}
}

func (m Model) importBodyView() string {
	st := m.ui.styles
	var b strings.Builder

	if m.importView.focus == importFocusPicker {
		b.WriteString(m.importView.picker.View())
		return b.String()
	}

	// Path input
	pathLabel := st.bodyBold.Render("File:")
	pathVal := m.importView.pathInput.View()
	b.WriteString(pathLabel + " " + pathVal + "\n")

	// Parse error
	if m.importView.parseError != "" {
		b.WriteString(st.bodyDim.Render(m.importView.parseError) + "\n")
		return b.String()
	}

	if len(m.importView.groups) == 0 {
		b.WriteString(st.bodyDim.Render("No profiles to show.") + "\n")
		return b.String()
	}

	// Group list
	b.WriteString("\n")
	visibleStart, visibleEnd := m.importVisibleRange()
	for i := range m.importView.groups {
		if i < visibleStart || i >= visibleEnd {
			continue
		}
		g := m.importView.groups[i]
		b.WriteString(m.renderImportGroupRow(i, g))
	}

	return b.String()
}

func (m Model) renderImportGroupRow(i int, g importGroup) string {
	st := m.ui.styles
	cursor := "  "
	if i == m.importView.cursor && m.importView.focus == importFocusList {
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
	total := len(m.importView.groups)
	if total == 0 {
		return 0, 0
	}
	maxVis := m.importMaxVisibleItems()
	if total <= maxVis {
		return 0, total
	}
	// Keep cursor in view
	if m.importView.cursor < m.importView.scrollOffset {
		m.importView.scrollOffset = m.importView.cursor
	}
	if m.importView.cursor >= m.importView.scrollOffset+maxVis {
		m.importView.scrollOffset = m.importView.cursor - maxVis + 1
	}
	if m.importView.scrollOffset < 0 {
		m.importView.scrollOffset = 0
	}
	if m.importView.scrollOffset > total-maxVis {
		m.importView.scrollOffset = total - maxVis
	}
	return m.importView.scrollOffset, m.importView.scrollOffset + maxVis
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

// withImportGroupsCloned returns a Model whose import groups are safe to write
// element-wise. See [Model.withExportItemsCloned] for why this is needed.
func (m Model) withImportGroupsCloned() Model {
	m.importView.groups = slices.Clone(m.importView.groups)
	return m
}
