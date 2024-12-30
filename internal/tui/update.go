// internal/tui/update.go
package tui

import (
	"fmt"
	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/pkg/camera"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle various message types
	case tea.WindowSizeMsg:
		// Root Component size
		m.width = msg.Width
		m.height = msg.Height
		// logging view
		m.logViewport.Width = msg.Width
		m.logViewport.Height = 10 // Set to 10 lines for logs
		m.logViewport.SetContent(strings.Join(m.logs, "\n"))

	case tickMsg:
		m.currentTime = time.Time(msg)
		return m, timeTickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Save the configuration
			config.Save(&m.config)
			// Quit the program
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
		case "c":
			if m.activeTab == cameraTab && m.cameraSetupStep == stepNoCameraConfigured {
				m.cameraSetupStep = stepScanningForCameras
				m.status = "Scanning for cameras..."
				devices, err := m.cameraManager.ScanDevices()
				if err != nil {
					m.status = fmt.Sprintf("Error scanning for cameras: %v", err)
					m.cameraMessages = append(m.cameraMessages, cameraMessage{
						text:      fmt.Sprintf("Error scanning for cameras: %v", err),
						timestamp: time.Now(),
						isError:   true,
					})
					m.cameraSetupStep = stepNoCameraConfigured
					return m, nil
				}
				m.availableCameras = make([]camera.Device, len(devices))
				for i, device := range devices {
					m.availableCameras[i] = device
				}

				if len(m.availableCameras) > 0 {
					m.selectedCamera = m.availableCameras[0]
					m.cameraSetupStep = stepSelectCamera
					m.status = "Select a camera to configure"
				} else {
					m.status = "No cameras found"
					m.cameraSetupStep = stepNoCameraConfigured
				}
			}

		case "up", "down":
			if m.activeTab == cameraTab && m.cameraSetupStep == stepSelectCamera && len(m.availableCameras) > 0 {
				currentIndex := -1
				for i, camera := range m.availableCameras {
					if camera == m.selectedCamera {
						currentIndex = i
						break
					}
				}
				if msg.String() == "up" {
					if currentIndex <= 0 {
						currentIndex = len(m.availableCameras) - 1
					} else {
						currentIndex--
					}
				} else {
					if currentIndex >= len(m.availableCameras)-1 {
						currentIndex = 0
					} else {
						currentIndex++
					}
				}
				m.selectedCamera = m.availableCameras[currentIndex]
			}

		case "enter":
			if m.activeTab == cameraTab {
				m.addCameraMessage(fmt.Sprintf("Enter pressed, current step: %v", m.cameraSetupStep), false)
				switch m.cameraSetupStep {
				case stepSelectCamera:
					if m.selectedCamera.ID != "" {
						m.addCameraMessage(fmt.Sprintf("Current Step before transition: %v", m.cameraSetupStep), false)

						m.cameraSetupStep = stepTestCamera
						m.addCameraMessage(fmt.Sprintf("Next Step after transition: %v", m.cameraSetupStep), false)
						m.status = "Testing camera..."
						m.addCameraMessage("Starting camera test", false)
						// Open the selected camera and start streaming
						err := m.cameraManager.OpenCamera(m.selectedCamera.ID, camera.StreamConfig{
							Width:     640,
							Height:    480,
							Framerate: 30,
						})
						if err != nil {
							m.addCameraMessage(fmt.Sprintf("Failed to open camera: %v", err), true)
						}

						stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
						if err == nil {
							m.addCameraMessage("Starting camera stream", false)
							go func() {
								m.addCameraMessage("Got stream successfully", false)
								for frame := range stream {
									m.server.BroadcastFrame(frame)
								}
								m.addCameraMessage("Camera stream ended", false)
							}()
						} else {
							m.addCameraMessage(fmt.Sprintf("Failed to start stream: %v", err), true)

						}
					}

				case stepTestCamera:
					m.addCameraMessage("In test step, starting stream", false)
					// Start streaming immediately when we enter test step
					stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
					if err == nil { // Changed condition, start stream on success
						m.addCameraMessage("Starting camera stream", false)
						go func() {
							for frame := range stream {
								m.server.BroadcastFrame(frame)
							}
							m.addCameraMessage("Camera stream ended", false)
						}()
					} else {
						m.addCameraMessage(fmt.Sprintf("Failed to start stream: %v", err), true)
					}

					// Handle enter press to move to next step
					if msg.String() == "enter" {
						m.cameraSetupStep = stepComplete
						m.status = "Configuring camera..."
						return m, nil
					}

				case stepConfigureCamera:
					m.cameraConfigured = true
					m.cameraSetupStep = stepComplete
					m.status = "Camera configured!"
					return m, nil

				case stepComplete:
					return m, nil

				}
			}
		case "b", "backspace", "esc":
			if m.activeTab == cameraTab {
				switch m.cameraSetupStep {
				case stepSelectCamera:
					m.cameraSetupStep = stepNoCameraConfigured
				case stepConfigureCamera:
					m.cameraSetupStep = stepTestCamera
				}
			}

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

		case "r":
			// Reset camera setup
			if m.activeTab == cameraTab && m.cameraConfigured {
				// Reset camera setup
				m.cameraSetupStep = stepNoCameraConfigured
				m.selectedCamera.ID = ""
				m.availableCameras = make([]camera.Device, 0)
				m.cameraConfigured = false
				m.status = "!! Camera setup reset !!"
			}
		case "tab":
			// Cycle through tabs
			m.activeTab = (m.activeTab + 1) % tabType(len(m.tabs))
		}
	}
	return m, nil
}
