package tui

import tea "charm.land/bubbletea/v2"

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func keyCtrl(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

func viewString(m interface{ View() tea.View }) string {
	return m.View().Content
}
