package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/pkg/camera"
	tea "github.com/charmbracelet/bubbletea"
)

// Update must have a pointer receiver if we want to mutate the same instance
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	//---------------------------------------------------------------------------
	case tea.WindowSizeMsg:
		// Keep track of app size
		m.width = msg.Width
		m.height = msg.Height

		// Adjust logging viewport
		m.logViewport.Width = msg.Width - 3
		m.logViewport.Height = 10
		// Refresh its content from m.logs
		m.logViewport.SetContent(strings.Join(m.logs, "\n"))
		return m, nil

	//---------------------------------------------------------------------------
	case tickMsg:
		m.currentTime = time.Time(msg)
		return m, timeTickCmd()

	//---------------------------------------------------------------------------
	case logUpdateMsg:
		// The logs were just flushed, so re-render with no extra side effects
		return m, nil

	//---------------------------------------------------------------------------
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Save config on quit
			config.Save(m.config)
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
			// Start camera setup
			if m.activeTab == cameraTab && m.cameraSetupStep == stepNoCameraConfigured {
				m.cameraSetupStep = stepScanningForCameras
				m.status = "Scanning for cameras..."
				devices, err := m.cameraManager.ScanDevices()
				if err != nil {
					m.status = fmt.Sprintf("Error scanning for cameras: %v", err)
					m.addCameraMessage(m.status, true)
					m.cameraSetupStep = stepNoCameraConfigured
					return m, nil
				}
				m.availableCameras = devices

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
			if m.activeTab == cameraTab &&
				m.cameraSetupStep == stepSelectCamera &&
				len(m.availableCameras) > 0 {

				currentIndex := 0
				for i, cam := range m.availableCameras {
					if cam == m.selectedCamera {
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
						m.addLog("INFO", "Moving from stepSelectCamera to stepTestCamera")
						m.cameraSetupStep = stepTestCamera
						m.status = "Testing camera..."
						// Try opening the camera
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
							m.addLog("INFO", "Camera opened successfully; starting stream goroutine")
							go func() {
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
					// Potentially test the camera stream again or finalize
					stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
					if err == nil {
						m.addLog("INFO", "Streaming test again on stepTestCamera -> stepComplete")
						go func() {
							for frame := range stream {
								m.server.BroadcastFrame(frame)
							}
							m.addLog("INFO", "Camera stream ended in test step")
						}()
					} else {
						m.addLog("ERROR", fmt.Sprintf("Failed to start stream in testCamera: %v", err))
					}

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
							m.addLog("INFO", "Camera is configured; starting final stream")
							go func() {
								for frame := range stream {
									m.server.BroadcastFrame(frame)
								}
							}()
						} else {
							m.addLog("ERROR", fmt.Sprintf("Failed to start final stream: %v", err))
						}
					}
					return m, nil
				}

			}

		case "b", "backspace", "esc":
			// Let’s allow going back in camera tab
			if m.activeTab == cameraTab {
				switch m.cameraSetupStep {
				case stepSelectCamera:
					m.cameraSetupStep = stepNoCameraConfigured
				case stepConfigureCamera:
					m.cameraSetupStep = stepTestCamera
				}
			}

		case "s":
			// Start/Stop server
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
			// Not implemented yet
			if m.activeTab == serverTab {
				// ...
			}

		case "r":
			// Reset camera setup
			if m.activeTab == cameraTab && m.cameraConfigured {
				m.cameraSetupStep = stepNoCameraConfigured
				m.selectedCamera.ID = ""
				m.availableCameras = []camera.Device{}
				m.cameraConfigured = false
				m.status = "!! Camera setup reset !!"
			}

		case "v":
			// Toggle verbosity
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
