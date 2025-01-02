// view.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---
var (
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3CB371")).
			Foreground(lipgloss.Color("#FFFFFF"))

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("250")).
			Padding(0, 1)

	mainContentStyle = lipgloss.NewStyle().
				Padding(1, 0)

	tabStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#98FB98")). // PaleGreen
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1)

	activeTabStyle = tabStyle.Copy().
			Background(lipgloss.Color("#228B22")). // ForestGreen
			Foreground(lipgloss.Color("#FFFFFF"))  // White text

	tabSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#556B2F")). // DarkOliveGreen
				SetString("|")

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Background(lipgloss.Color("#333333")).
			Padding(1, 2)

	styleBoxed = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			BorderForeground(lipgloss.Color("79"))
)

// View renders the TUI
func (m *Model) View() string {
	timeStr := m.currentTime.Format("Mon Jan 2 15:04:05 2006")

	//----------------------------------------------------------------------
	// 1. Header at the very top
	//----------------------------------------------------------------------
	leftText := "🐦 FeatherFrame "
	leftWidth := lipgloss.Width(leftText)
	availableWidth := m.width - leftWidth
	if availableWidth < 0 {
		// If terminal is super narrow, avoid negative widths
		availableWidth = 0
	}

	headerContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		headerStyle.Render(leftText),
		headerStyle.Copy().
			Width(availableWidth).
			Align(lipgloss.Right).
			Render(timeStr),
	)
	headerRendered := headerStyle.Copy().
		Width(m.width).
		Render(headerContent)

	//----------------------------------------------------------------------
	// 2. Tabs right below the header
	//----------------------------------------------------------------------
	tabsRendered := m.renderTabs()

	//----------------------------------------------------------------------
	// 3. Main content
	//----------------------------------------------------------------------
	mainContent := m.renderActiveTabContent()
	mainContentRendered := mainContentStyle.
		Width(m.width).
		Render(mainContent)

	//----------------------------------------------------------------------
	// 4. Logs: FIXED at 10 lines, never grows.
	//    Let the user scroll in those 10 lines if needed.
	//----------------------------------------------------------------------
	const logBoxHeight = 10
	logsRendered := footerStyle.
		Width(m.width-3).
		Height(logBoxHeight). // Hard-coded to 10 lines
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("#FFA500")).
		Padding(1).
		Render(m.logViewport.View())

	//----------------------------------------------------------------------
	// 5. Status bar at the bottom
	//----------------------------------------------------------------------
	statusBarRendered := statusBarStyle.
		Width(m.width).
		Render(fmt.Sprintf("Status: %s | Tab or Num 1-4: Switch Views | Press q to quit", m.status))

	//----------------------------------------------------------------------
	// 6. Join everything vertically in the final layout
	//----------------------------------------------------------------------
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerRendered, // top
		tabsRendered,   // below header
		mainContentRendered,
		logsRendered,      // 10-line logs
		statusBarRendered, // bottom
	)
}

// renderLogFooter (unused in this approach, but kept in case you need it)
// func (m *Model) renderLogFooter() string {
// 	boxWidth := m.width - 3
// 	return footerStyle.
// 		Width(boxWidth).
// 		Border(lipgloss.RoundedBorder(), true).
// 		BorderForeground(lipgloss.Color("#FFA500")).
// 		Padding(1).
// 		Render(m.logViewport.View())
// }

// renderTabs
func (m *Model) renderTabs() string {
	var rendered []string
	for _, t := range m.tabs {
		style := tabStyle
		if t.id == m.activeTab {
			style = activeTabStyle
		}
		rendered = append(rendered, style.Render(t.title))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// renderActiveTabContent
func (m *Model) renderActiveTabContent() string {
	switch m.activeTab {
	case cameraTab:
		return m.renderCameraContent()
	case motionTab:
		return "Motion Detection:\n• Status: Active\n• Sensitivity: Medium\n• Events Today: 0"
	case classificationTab:
		return "Bird Classification:\n• Model: Loaded\n• Detections: 0\n• Confidence Threshold: 0.8"
	case storageTab:
		return "Storage Status:\n• Path: /home/pi/birdwatcher\n• Total: 100GB\n• Used: 10GB\n• Free: 90GB"
	case serverTab:
		status := "Stopped"
		if m.server.IsRunning() {
			status = fmt.Sprintf("Running on port %s", m.serverPort)
		}
		return fmt.Sprintf(
			"Web Server Status:\n• Status: %s\n• Port: %s\n• Press 's' to start/stop server\n• Press 'p' to change port\n",
			status, m.server.Port(),
		)
	}
	return ""
}

// renderCameraContent
func (m *Model) renderCameraContent() string {
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
		return "Scanning for cameras...\nThis may take a few moments..."

	case stepSelectCamera:
		var sb strings.Builder
		sb.WriteString("Available Cameras:\n\n")
		for i, cam := range m.availableCameras {
			cursor := "  "
			if cam == m.selectedCamera {
				cursor = "→ "
			}
			sb.WriteString(fmt.Sprintf("%s%d: %s (%s)\n", cursor, i+1, cam.Name, cam.ID))
		}
		sb.WriteString("\nUse ↑/↓ to select, Enter to confirm.")
		return sb.String()

	case stepTestCamera:
		return styleBoxed.Render(
			fmt.Sprintf(
				"Testing Camera: %s\n\n"+
					"• Preview at: http://localhost:%s/setup-preview\n"+
					"• Press Enter to continue if good\n"+
					"• Press 'b' to go back to camera selection",
				m.selectedCamera.Name, m.server.Port()),
		)

	case stepConfigureCamera:
		return "Applying camera configuration..."

	case stepComplete:
		return fmt.Sprintf(
			"Camera Status: Active\n"+
				"Device: %s\n"+
				"Resolution: 640x480\n"+
				"FPS: 30\n"+
				"Preview: http://localhost:%s/camera\n"+
				"Press 'r' to remove camera config",
			m.selectedCamera.Name, m.server.Port(),
		)
	}
	return ""
}
