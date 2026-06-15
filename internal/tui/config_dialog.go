package tui

import (
	"strconv"
	"strings"

	"treehole/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfigSection int

const (
	ConfigSectionAuth ConfigSection = iota
	ConfigSectionDatabase
)

type configFieldDef struct {
	label       string
	placeholder string
	secret      bool
}

type ConfigDialogModel struct {
	authInputs     []textinput.Model
	databaseInputs []textinput.Model
	section        ConfigSection
	focus          int
	saving         bool
	saveOK         bool
	lastErr        string
	formViewport   viewport.Model
	formContent    string
}

var authFieldDefs = []configFieldDef{
	{label: "用户名:", placeholder: "用户名"},
	{label: "密码:", placeholder: "密码", secret: true},
	{label: "SecretKey:", placeholder: "SecretKey", secret: true},
	{label: "DeviceUUID:", placeholder: "设备 UUID"},
}

var databaseFieldDefs = []configFieldDef{
	{label: "Type:", placeholder: "sqlite3/postgres"},
	{label: "Host:", placeholder: "localhost"},
	{label: "Port:", placeholder: "5432"},
	{label: "User:", placeholder: "数据库用户名"},
	{label: "Password:", placeholder: "数据库密码", secret: true},
	{label: "Name:", placeholder: "数据库名"},
	{label: "DBFile:", placeholder: "./treehole.db"},
	{label: "SSLMode:", placeholder: "disable"},
	{label: "DSN:", placeholder: "自定义 DSN"},
}

func newConfigInputs(defs []configFieldDef, values []string) []textinput.Model {
	inputs := make([]textinput.Model, len(defs))
	for i, def := range defs {
		input := textinput.New()
		input.Prompt = ""
		input.Placeholder = def.placeholder
		if i < len(values) {
			input.SetValue(values[i])
		}
		input.Width = 40
		if def.secret {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		inputs[i] = input
	}
	return inputs
}

func NewConfigDialog(cfg *config.Config) ConfigDialogModel {
	authValues := []string{"", "", "", ""}
	dbValues := []string{"", "", "", "", "", "", "", "", ""}
	if cfg != nil {
		authValues = []string{cfg.Username, cfg.Password, cfg.SecretKey, cfg.DeviceUUID}
		dbValues = []string{
			cfg.Database.Type,
			cfg.Database.Host,
			portToString(cfg.Database.Port),
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Name,
			cfg.Database.DBFile,
			cfg.Database.SSLMode,
			cfg.Database.DSN,
		}
	}
	m := ConfigDialogModel{
		authInputs:     newConfigInputs(authFieldDefs, authValues),
		databaseInputs: newConfigInputs(databaseFieldDefs, dbValues),
		section:        ConfigSectionAuth,
		formViewport:   viewport.New(0, 0),
	}
	m.setFocus(0)
	return m
}

func portToString(port int) string {
	if port == 0 {
		return ""
	}
	return strconv.Itoa(port)
}

func (m ConfigDialogModel) initialized() bool {
	return len(m.authInputs) == len(authFieldDefs) && len(m.databaseInputs) == len(databaseFieldDefs)
}

func (m ConfigDialogModel) currentInputs() []textinput.Model {
	if m.section == ConfigSectionDatabase {
		return m.databaseInputs
	}
	return m.authInputs
}

func (m *ConfigDialogModel) currentInputsRef() *[]textinput.Model {
	if m.section == ConfigSectionDatabase {
		return &m.databaseInputs
	}
	return &m.authInputs
}

func (m ConfigDialogModel) currentFieldDefs() []configFieldDef {
	if m.section == ConfigSectionDatabase {
		return databaseFieldDefs
	}
	return authFieldDefs
}

func (m ConfigDialogModel) saveIndex() int {
	return len(m.currentInputs())
}

func (m *ConfigDialogModel) setFocus(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx > m.saveIndex() {
		idx = m.saveIndex()
	}
	m.focus = idx
	inputs := m.currentInputsRef()
	for i := range *inputs {
		if i == idx {
			(*inputs)[i].Focus()
		} else {
			(*inputs)[i].Blur()
		}
	}
}

func (m *ConfigDialogModel) switchSection(section ConfigSection) {
	if m.section == section {
		return
	}
	m.section = section
	m.focus = 0
	m.setFocus(0)
}

func (m *ConfigDialogModel) SetConfig(cfg *config.Config) {
	if !m.initialized() {
		*m = NewConfigDialog(cfg)
		return
	}
	if cfg == nil {
		return
	}
	values := [][]string{
		{cfg.Username, cfg.Password, cfg.SecretKey, cfg.DeviceUUID},
		{
			cfg.Database.Type,
			cfg.Database.Host,
			portToString(cfg.Database.Port),
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Name,
			cfg.Database.DBFile,
			cfg.Database.SSLMode,
			cfg.Database.DSN,
		},
	}
	for i := range m.authInputs {
		m.authInputs[i].SetValue(values[0][i])
	}
	for i := range m.databaseInputs {
		m.databaseInputs[i].SetValue(values[1][i])
	}
	m.saveOK = false
	m.lastErr = ""
	m.switchSection(ConfigSectionAuth)
}

func (m *ConfigDialogModel) SetSaving(saving bool) {
	m.saving = saving
}

func (m *ConfigDialogModel) SetSaveResult(err error) {
	m.saving = false
	if err != nil {
		m.saveOK = false
		m.lastErr = err.Error()
		return
	}
	m.saveOK = true
	m.lastErr = ""
}

func (m *ConfigDialogModel) FocusIndex() int {
	return m.focus
}

func (m *ConfigDialogModel) IsSaveFocused() bool {
	return m.focus == m.saveIndex()
}

func (m *ConfigDialogModel) ActiveSection() ConfigSection {
	return m.section
}

func (m *ConfigDialogModel) Username() string   { return m.authInputs[0].Value() }
func (m *ConfigDialogModel) Password() string   { return m.authInputs[1].Value() }
func (m *ConfigDialogModel) SecretKey() string  { return m.authInputs[2].Value() }
func (m *ConfigDialogModel) DeviceUUID() string { return m.authInputs[3].Value() }

func (m *ConfigDialogModel) ToConfig(existing *config.Config) *config.Config {
	result := &config.Config{}
	if existing != nil {
		*result = *existing
	}
	result.Username = m.Username()
	result.Password = m.Password()
	result.SecretKey = m.SecretKey()
	result.DeviceUUID = m.DeviceUUID()
	result.Database.Type = m.databaseInputs[0].Value()
	result.Database.Host = m.databaseInputs[1].Value()
	portText := strings.TrimSpace(m.databaseInputs[2].Value())
	if port, err := strconv.Atoi(portText); err == nil {
		result.Database.Port = port
		if portText == "" {
			result.Database.Port = 0
		}
	} else if portText == "" {
		result.Database.Port = 0
	}
	result.Database.User = m.databaseInputs[3].Value()
	result.Database.Password = m.databaseInputs[4].Value()
	result.Database.Name = m.databaseInputs[5].Value()
	result.Database.DBFile = m.databaseInputs[6].Value()
	result.Database.SSLMode = m.databaseInputs[7].Value()
	result.Database.DSN = m.databaseInputs[8].Value()
	return result
}

func (m *ConfigDialogModel) Update(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEscape:
		return nil
	case tea.KeyUp:
		if m.focus > 0 {
			m.setFocus(m.focus - 1)
		}
		return nil
	case tea.KeyDown:
		if m.focus < m.saveIndex() {
			m.setFocus(m.focus + 1)
		}
		return nil
	case tea.KeyEnter:
		if m.focus < m.saveIndex() {
			m.setFocus(m.saveIndex())
		}
		return nil
	case tea.KeyLeft:
		if m.IsSaveFocused() {
			m.switchSection(ConfigSectionAuth)
			return nil
		}
	case tea.KeyRight:
		if m.IsSaveFocused() {
			m.switchSection(ConfigSectionDatabase)
			return nil
		}
	}

	if m.focus < m.saveIndex() {
		inputs := m.currentInputsRef()
		var cmd tea.Cmd
		(*inputs)[m.focus], cmd = (*inputs)[m.focus].Update(msg)
		return cmd
	}
	return nil
}

func (m ConfigDialogModel) sectionTitle() string {
	if m.section == ConfigSectionDatabase {
		return "database"
	}
	return "auth"
}

func (m ConfigDialogModel) renderSidebar() string {
	items := []struct {
		label  string
		active bool
	}{
		{"账号/认证\n", m.section == ConfigSectionAuth},
		{"数据库\n", m.section == ConfigSectionDatabase},
	}
	var lines []string
	for _, item := range items {
		prefix := "  "
		style := vStatLabelStyle
		if item.active {
			prefix = "→ "
			style = vStatValueStyle
		}
		lines = append(lines, style.Render(prefix+item.label))
	}
	return strings.Join(lines, "\n")
}

func (m ConfigDialogModel) dialogContentWidth(totalWidth int) int {
	return minInt(70, maxInt(40, totalWidth-8))
}

func (m ConfigDialogModel) fieldBoxContentWidth(dialogWidth int) int {
	return minInt(40, maxInt(24, dialogWidth-30))
}

func (m ConfigDialogModel) fieldValueDisplay(input textinput.Model, def configFieldDef, focused bool) string {
	value := input.Value()
	if def.secret {
		value = strings.Repeat("*", len([]rune(value)))
	}
	if !focused {
		return value
	}

	runes := []rune(value)
	pos := input.Position()
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}

	withCursor := make([]rune, 0, len(runes)+1)
	withCursor = append(withCursor, runes[:pos]...)
	withCursor = append(withCursor, '█')
	withCursor = append(withCursor, runes[pos:]...)
	return string(withCursor)
}

func (m ConfigDialogModel) renderFieldBox(input textinput.Model, def configFieldDef, boxContentWidth int, focused bool) string {
	prefixWidth := lipgloss.Width(def.label) + 1
	valueWidth := maxInt(1, boxContentWidth-prefixWidth)
	valueLines := wrapVisibleLine(m.fieldValueDisplay(input, def, focused), valueWidth)
	if len(valueLines) == 0 {
		valueLines = []string{""}
	}

	contentLines := make([]string, 0, len(valueLines))
	for i, line := range valueLines {
		prefix := strings.Repeat(" ", prefixWidth)
		if i == 0 {
			prefix = def.label + strings.Repeat(" ", maxInt(0, prefixWidth-lipgloss.Width(def.label)))
		}
		row := lipgloss.NewStyle().
			Width(boxContentWidth).
			Render(prefix + line)
		contentLines = append(contentLines, row)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder)
	if focused {
		boxStyle = boxStyle.BorderForeground(colorAccent)
	}

	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

func (m *ConfigDialogModel) syncFormViewport(width, height int, content string, focusRanges [][2]int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if m.formViewport.Width != width || m.formViewport.Height != height || m.formContent != content {
		m.formViewport = viewport.New(width, height)
		m.formViewport.SetContent(content)
		m.formContent = content
	}
	if m.focus >= 0 && m.focus < len(focusRanges) {
		top := focusRanges[m.focus][0]
		bottom := focusRanges[m.focus][1]
		viewTop := m.formViewport.YOffset
		viewBottom := viewTop + m.formViewport.Height - 1
		if top < viewTop {
			m.formViewport.SetYOffset(top)
		} else if bottom > viewBottom {
			m.formViewport.SetYOffset(maxInt(0, bottom-m.formViewport.Height+1))
		}
	}
}

func (m *ConfigDialogModel) View(width, height int) string {
	var b strings.Builder

	dialogWidth := m.dialogContentWidth(width)
	boxContentWidth := m.fieldBoxContentWidth(dialogWidth)

	b.WriteString(vDialogTitleStyle.Render("配置管理"))
	b.WriteString("\n\n")
	b.WriteString(vSubtitleStyle.Render("data/config.json / " + m.sectionTitle()))
	b.WriteString("\n\n")

	fieldDefs := m.currentFieldDefs()
	inputs := m.currentInputs()

	var form strings.Builder
	var focusRanges [][2]int
	currentLine := 0
	for i, f := range fieldDefs {
		block := m.renderFieldBox(inputs[i], f, boxContentWidth, m.focus == i)
		blockHeight := lipgloss.Height(block)
		focusRanges = append(focusRanges, [2]int{currentLine, currentLine + blockHeight - 1})
		form.WriteString(block)
		form.WriteString("\n\n")
		currentLine += blockHeight + 2
	}

	saveBtn := "保存配置"
	saveRendered := vFormSaveBtn.Render(saveBtn)
	if m.IsSaveFocused() {
		saveRendered = vFormSaveActive.Render(saveBtn)
	}
	saveLine := lipgloss.NewStyle().
		Width(boxContentWidth + 2).
		Align(lipgloss.Right).
		Render(saveRendered)
	saveHeight := lipgloss.Height(saveLine)
	focusRanges = append(focusRanges, [2]int{currentLine, currentLine + saveHeight - 1})
	form.WriteString(saveLine)

	formWidth := boxContentWidth + 2
	helpText := vDialogHelpStyle.Render("按钮上 ← →: 切换页面 | Esc: 关闭")
	headerHeight := lipgloss.Height(b.String())
	statusHeight := 0
	if m.saving {
		statusHeight += lipgloss.Height(vLoadingStyle.Render("保存中..."))
	}
	if m.saveOK {
		statusHeight += lipgloss.Height(vStatusRunningStyle.Render("配置已保存!"))
	}
	if m.lastErr != "" {
		statusHeight += lipgloss.Height(vErrorStyle.Render("错误: " + m.lastErr))
	}
	footerHeight := lipgloss.Height(helpText) + 2
	bodyHeight := maxInt(3, height-dialogCard.GetVerticalFrameSize()-headerHeight-statusHeight-footerHeight)
	m.syncFormViewport(formWidth, bodyHeight, form.String(), focusRanges)

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(12).Render(m.renderSidebar()),
		"   ",
		m.formViewport.View(),
	)
	b.WriteString(body)

	if m.saving {
		b.WriteString("\n")
		b.WriteString(vLoadingStyle.Render("保存中..."))
	}
	if m.saveOK {
		b.WriteString("\n")
		b.WriteString(vStatusRunningStyle.Render("配置已保存!"))
	}
	if m.lastErr != "" {
		b.WriteString("\n")
		b.WriteString(vErrorStyle.Render("错误: " + m.lastErr))
	}

	b.WriteString("\n\n")
	b.WriteString(helpText)
	return b.String()
}

func maskField(s string, mask bool) string {
	if s == "" {
		return "(空)"
	}
	if mask {
		return strings.Repeat("*", len(s))
	}
	return s
}
