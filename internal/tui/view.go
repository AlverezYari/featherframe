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

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Background(lipgloss.Color("#333333")).
			Padding(1, 2)
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

	// Logging footer
	logs := m.renderLogFooter()

	// Status bar
	statusBar := statusBarStyle.Width(m.width).Render(
		fmt.Sprintf("Status: %s | Tab or Num 1-4: Switch Views | Press q to quit", m.status),
	)

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		mainContent,
		logs,
		statusBar,
	)
}

// Logging functions
func (m *Model) renderLogFooter() string {
	boxWidth := m.width - 3

	// Use the viewport content to render the logs
	return footerStyle.
		Width(boxWidth).
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("#FFA500")).
		Padding(1).
		Render(m.logViewport.View())
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
		return m.renderCameraContent()
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
	case storageTab:
		return "Storage Status:\n" +
			"• Path: /home/pi/birdwatcher\n" +
			"• Total: 100GB\n" +
			"• Used: 10GB\n" +
			"• Free: 90GB"
	case serverTab:
		var content strings.Builder

		status := "Stopped"
		if m.server.IsRunning() {
			status = fmt.Sprintf("Running on port %s", m.serverPort)
		}

		content.WriteString(fmt.Sprintf(
			"Web Server Status:\n"+
				"• Status: %s\n"+
				"• Port: %s\n"+
				"• Press 's' to start/stop server\n"+
				"• Press 'p' to change port\n\n",
			status, m.server.Port(),
		))

		// Return server status content (logs are now in the unified log box)
		return content.String()
	}

	return ""
}

// CHANGED: Update camera tab content
func (m Model) renderCameraMainContent() string {
	// if we have a configured camera show that first!
	if m.cameraConfigured {
		return styleBoxed.Render(
			fmt.Sprintf(
				"Camera Status: Active\n"+
					"Device: %s\n"+
					"Resolution: %s\n"+
					"FPS: %d\n"+
					"Live Monitor: http://localhost:%s/live-monitor\n"+
					"Press 'r' to remove the camera configuration",
				m.config.CameraConfig.DeviceName,
				m.config.CameraConfig.StreamConfig.Resolution,
				m.config.CameraConfig.StreamConfig.FPS,
				m.server.Port(),
			),
		)
	}
	switch m.cameraSetupStep {
	case stepNoCameraConfigured:
		return styleBoxed.Render(
			"No Camera Configured\n\n" +
				"Press 'c' to start camera setup wizard\n" +
				"This will:\n" +
				"• Scan for available cameras\n" +
				"• Help you select and test a camera\n" +
				"• Configure camera settings")

	case stepScanningForCameras:
		return "Scanning for cameras...\n" +
			"This may take a few moments..."

	case stepSelectCamera:
		var content strings.Builder
		content.WriteString("Available Cameras:\n\n")

		for i, camera := range m.availableCameras {
			prefix := "  "
			if camera == m.selectedCamera {
				prefix = "→ "
			}
			content.WriteString(fmt.Sprintf("%s%d: %s (%s)\n", prefix, i+1, camera.Name, camera.ID))
		}

		content.WriteString("\nUse ↑/↓ to select, Enter to confirm")
		return content.String()

	case stepTestCamera:
		return styleBoxed.Render(
			fmt.Sprintf("Testing Camera: %s\n\n"+
				"• Preview available at: http://localhost:%s/setup-preview\n"+
				"• Press Enter to continue with the config if preview looks good\n"+
				"• Press 'b' to go back to camera selection",
				m.selectedCamera.Name, m.server.Port()))

	case stepConfigureCamera:
		return "Applying camera configuration..."

	case stepComplete:
		return fmt.Sprintf(
			"Camera Status: Active\n"+
				"Device: %s\n"+
				"Resolution: 1080p\n"+
				"FPS: 30\n"+
				"Preview: http://localhost:%s/camera\n"+
				"Press 'r' to remove the camera configuration",
			m.selectedCamera.Name,
			m.server.Port())
	}

	return ""
}

func (m Model) renderCameraContent() string {
	// Get the main content for the camera tab
	return m.renderCameraMainContent()
}

// Added some styling for our camera tab, logging and content
var styleBoxed = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Padding(1).
	BorderForeground(lipgloss.Color("62"))

var (
	styleNormalMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	styleErrorMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("red"))
)
