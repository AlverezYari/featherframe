// internal/tui/view.go
package tui

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// Style definitions
var (
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)

	mainContentStyle = lipgloss.NewStyle().
				Padding(1, 0)

	tabStyle = lipgloss.NewStyle().
			Padding(0, 1)

	activeTabStyle = tabStyle.Copy().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("0"))

	tabSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("237")).
				SetString("|")
)

// View renders the UI
func (m Model) View() string {
	timeStr := m.currentTime.Format("Mon Jan 2 15:04:05 2006")

	// Header with tabs
	headerContent := lipgloss.JoinHorizontal(
		lipgloss.Center,
		"🐦 Birdwatcher",
		lipgloss.NewStyle().
			Width(m.width-18).
			Align(lipgloss.Right).
			Render(timeStr),
	)

	header := headerStyle.Width(m.width).Render(headerContent)

	// Tabs
	tabs := m.renderTabs()

	// Main content from active tab
	mainContent := mainContentStyle.Render(m.renderActiveTabContent())

	// Status bar
	statusBar := statusBarStyle.Width(m.width).Render(
		fmt.Sprintf("Status: %s | Tab or Num 1-4: Switch Views | Press q to quit", m.status),
	)

	// Combine all sections
	return fmt.Sprintf("%s\n%s\n%s\n%s", header, tabs, mainContent, statusBar)
}

// Helper function to render tabs
func (m Model) renderTabs() string {
	var renderedTabs []string

	for _, t := range m.tabs {
		style := tabStyle
		if t.id == m.activeTab {
			style = activeTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(t.title))
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderedTabs...,
	)
}

// Helper function to render active tab content
func (m Model) renderActiveTabContent() string {
	switch m.activeTab {
	case cameraTab:
		return "Camera Status:\n" +
			"• Resolution: 1080p\n" +
			"• FPS: 30\n" +
			"• Mode: Preview"
	case motionTab:
		return "Motion Detection:\n" +
			"• Status: Active\n" +
			"• Sensitivity: Medium\n" +
			"• Events Today: 0"
	case classificationTab:
		return "Bird Classification:\n" +
			"• Model: Loaded\n" +
			"• Detections: 0\n" +
			"• Confidence Threshold: 0.8"
	case uploadTab:
		return "Upload Status:\n" +
			"• Queue: Empty\n" +
			"• Last Upload: Never\n" +
			"• Storage Used: 0MB"
	case serverTab:
		var content strings.Builder

		status := "Stopped"
		if m.server.IsRunning() {
			status = fmt.Sprintf("Running on port %s", m.serverPort)
		}
		content.WriteString(fmt.Sprintf("Web Server Status:\n"+
			"• Status: %s\n"+
			"• Port: %s\n"+
			"• Press 's' to start/stop server\n"+
			"• Press 'p' to change port\n\n", status, m.server.Port()))
		content.WriteString("Recent Logs:\n")
		content.WriteString("------------\n")

		logs := m.server.GetRecentLogs(10)
		for _, entry := range logs {
			content.WriteString(fmt.Sprintf("%s\n",
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("241")).
					Render(entry.Message)))
		}

		return content.String()
	}
	return ""
}
