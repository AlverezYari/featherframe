// internal/tui/update.go
package tui

import (
	"fmt"
	"github.com/AlverezYari/featherframe/pkg/camera"
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
				m.availableCameras = make([]string, len(devices))
				for i, device := range devices {
					m.availableCameras[i] = device.Name
				}

				if len(m.availableCameras) > 0 {
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
				switch m.cameraSetupStep {
				case stepSelectCamera:
					if m.selectedCamera != "" {
						config := camera.StreamConfig{
							Width:     1280,
							Height:    720,
							Framerate: 30,
							Mode:      camera.ModeLiveMonitor,
						}

						if err := m.cameraManager.OpenCamera(m.selectedCamera, config); err != nil {
							m.addCameraMessage(fmt.Sprintf("Error opening camera: %v", err), true)
							return m, nil
						}
						m.addCameraMessage("Camera openned successfully!", false)
						m.cameraSetupStep = stepTestCamera
						m.status = "Testing camera..."
					}

				case stepTestCamera:
					if msg.String() == "enter" {
						m.cameraSetupStep = stepConfigureCamera
						m.cameraConfigured = true
						m.status = "Configuring camera..."
					} else {
						fmt.Println("Attempting to start stream")
						if stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera); err == nil {
							fmt.Println("Got stream starting broadcast")
							go func() {
								for frame := range stream {
									m.server.BroadcastFrame(frame)
								}
							}()
						} else {
							fmt.Println("Error starting stream")
							m.status = fmt.Sprintf("Error starting stream: %v", err)
							m.cameraMessages = append(m.cameraMessages, cameraMessage{
								text:      fmt.Sprintf("Error starting stream: %v", err),
								timestamp: time.Now(),
								isError:   true,
							})
						}
					}

				case stepConfigureCamera:
					m.cameraSetupStep = stepComplete
					m.cameraConfigured = true
					m.status = "Camera configured!"
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
				m.selectedCamera = ""
				m.availableCameras = make([]string, 0)
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
