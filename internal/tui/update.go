// internal/tui/update.go
package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle various message types
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.currentTime = time.Time(msg)
		return m, timeTickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.activeTab = cameraTab
		case "2":
			m.activeTab = motionTab
		case "3":
			m.activeTab = classificationTab
		case "4":
			m.activeTab = uploadTab
		case "5":
			m.activeTab = serverTab
		case "s":
			if m.activeTab == serverTab {
				if m.server.IsRunning() {
					if err := m.server.Stop(); err != nil {
						m.status = fmt.Sprintf("Error stopping server: %v", err)
					} else {
						m.status = "Server stopped"
					}
				} else {
					if err := m.server.Start(); err != nil {
						m.status = fmt.Sprintf("Error starting server: %v", err)
					} else {
						m.status = fmt.Sprintf("Server started on port %s", m.server.Port())
					}
				}
			}
		case "p":
			if m.activeTab == serverTab {
				// TODO: Implement changing server port
			}

		case "tab":
			// Cycle through tabs
			m.activeTab = (m.activeTab + 1) % tabType(len(m.tabs))
		}
	}
	return m, nil
}
