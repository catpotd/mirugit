package tui

import tea "charm.land/bubbletea/v2"

func (m *Model) checkForUpdate() tea.Cmd {
	check := m.updateCheck
	if check == nil {
		return nil
	}
	ctx := m.rootContext()
	return func() tea.Msg {
		version, err := check(ctx)
		if err != nil {
			return updateAvailableMsg{}
		}
		return updateAvailableMsg{version: version}
	}
}
