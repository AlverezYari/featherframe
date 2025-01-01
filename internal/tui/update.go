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

	case logUpdateMsg:
		// Refresh view when a log update occurs
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Save the configuration
			config.Save(m.config)
			// Quit the program
			return m, tea.Quit

		case "1":
			m.activeTab = cameraTab
		case "2":
			m.activeTab = motionTab
		case "3":
			m.activeTab = classificationTab
		case "4":
			m.activeTab = storageTab
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
				m.addLog("INFO", fmt.Sprintf("Enter pressed, current step: %v", m.cameraSetupStep))
				switch m.cameraSetupStep {
				case stepSelectCamera:
					if m.selectedCamera.ID != "" {
						m.addLog("INFO", fmt.Sprintf("Current Step before transition: %v", m.cameraSetupStep))

						m.cameraSetupStep = stepTestCamera
						m.addLog("INFO", fmt.Sprintf("Next Step after transition: %v", m.cameraSetupStep))
						m.status = "Testing camera..."
						m.addLog("INFO", "Starting camera test")
						// Open the selected camera and start streaming
						err := m.cameraManager.OpenCamera(m.selectedCamera.ID, camera.StreamConfig{
							Width:     640,
							Height:    480,
							Framerate: 30,
						})
						if err != nil {
							m.addLog("ERROR", fmt.Sprintf("Failed to open camera: %v", err))
						}

						stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
						if err == nil {
							m.addLog("INFO", "Starting camera stream")
							go func() {
								m.addLog("INFO", "Got stream successfully")
								for frame := range stream {
									m.server.BroadcastFrame(frame)
								}
								m.addLog("INFO", "Camera stream ended")
							}()
						} else {
							m.addLog("ERROR", fmt.Sprintf("Failed to start stream: %v", err))

						}
					}

				case stepTestCamera:
					m.addLog("INFO", "In test step, starting stream")
					// Start streaming immediately when we enter test step
					stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
					if err == nil { // Changed condition, start stream on success
						m.addLog("INFO", "Starting camera stream")
						go func() {
							for frame := range stream {
								m.server.BroadcastFrame(frame)
							}
							m.addLog("INFO", "Camera stream ended")
						}()
					} else {
						m.addLog("ERROR", fmt.Sprintf("Failed to start stream: %v", err))
					}

					// Handle enter press to move to next step
					if msg.String() == "enter" {
						m.cameraSetupStep = stepComplete
						m.status = "Configuring camera..."
						m.config.CameraConfig = config.CameraConfig{
							DeviceID:   m.selectedCamera.ID,
							DeviceName: m.selectedCamera.Name,
							StreamConfig: config.StreamConfig{
								Resolution: "640x480",
								FPS:        30,
							},
						}

						return m, nil
					}

				case stepConfigureCamera:
					m.cameraConfigured = true
					m.cameraSetupStep = stepComplete
					m.status = "Camera configured!"
					return m, nil

				case stepComplete:
					config.Save(m.config)
					if m.cameraConfigured {
						stream, err := m.cameraManager.GetStreamChannel(m.config.CameraConfig.DeviceID)
						if err == nil {
							go func() {
								for frame := range stream {
									m.server.BroadcastFrame(frame)
								}
							}()
						} else {
							m.addLog("ERROR", fmt.Sprintf("Failed to start stream: %v", err))
						}
					}

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
		case "v": // Toggle logging verbosity
			m.verbosity++
			if m.verbosity > VerbosityDebug {
				m.verbosity = VerbosityError
			}
			switch m.verbosity {
			case VerbosityError:
				m.status = "Verbosity: ERROR"
			case VerbosityInfo:
				m.status = "Verbosity: INFO"
			case VerbosityDebug:
				m.status = "Verbosity: DEBUG"
			}

			return m, nil

		case "tab":
			// Cycle through tabs
			m.activeTab = (m.activeTab + 1) % tabType(len(m.tabs))
		}
	}
	return m, nil
}
