package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/pkg/camera"
	tea "github.com/charmbracelet/bubbletea"
)

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

		//-----------------------------------------------------------------------
		// Quit
		case "q", "ctrl+c":
			// Save config on quit
			config.Save(m.config)
			return m, tea.Quit

		//-----------------------------------------------------------------------
		// Tab switching or numeric switching
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

		//-----------------------------------------------------------------------
		// 'c': Start camera setup wizard
		case "c":
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
					m.status = "Select a camera to configure (↑/↓, Enter)"
				} else {
					m.status = "No cameras found"
					m.cameraSetupStep = stepNoCameraConfigured
				}
			}

		//-----------------------------------------------------------------------
		// Arrow keys to pick a camera
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
				} else { // down
					if currentIndex >= len(m.availableCameras)-1 {
						currentIndex = 0
					} else {
						currentIndex++
					}
				}
				m.selectedCamera = m.availableCameras[currentIndex]
				m.status = fmt.Sprintf("Selected: %s", m.selectedCamera.Name)
			}

		//-----------------------------------------------------------------------
		// Enter key pressed
		case "enter":
			if m.activeTab == cameraTab {
				m.addLog("INFO", fmt.Sprintf("Enter pressed, current step: %v", m.cameraSetupStep))

				switch m.cameraSetupStep {

				//-------------------------------------------------------------------
				// 1) stepSelectCamera -> open camera + stream -> stepTestCamera
				case stepSelectCamera:
					if m.selectedCamera.ID == "" {
						return m, nil
					}
					m.addLog("INFO", "Opening camera + starting stream (selectCamera)")

					// Open the camera
					err := m.cameraManager.OpenCamera(
						m.selectedCamera.ID,
						camera.StreamConfig{Width: 640, Height: 480, Framerate: 30},
					)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to open camera: %v", err))
						return m, nil
					}

					// Start the stream
					stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to get stream: %v", err))
						return m, nil
					}

					go func() {
						for frame := range stream {
							m.server.BroadcastFrame(frame)
						}
						m.addLog("INFO", "Camera stream ended (stepSelectCamera -> closed)")
					}()

					// Move to testCamera
					m.cameraSetupStep = stepTestCamera
					m.status = "Camera is streaming; press Enter again to confirm if it looks good."
					return m, nil

				//-------------------------------------------------------------------
				// 2) stepTestCamera -> confirm => close camera => config -> stepComplete
				case stepTestCamera:
					m.addLog("INFO", "User confirmed camera is good. Finalizing setup.")

					// Close the camera to stop streaming
					err := m.cameraManager.CloseCamera(m.selectedCamera.ID)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to close camera: %v", err))
					}

					// Save to config
					m.config.CameraConfig = config.CameraConfig{
						DeviceID:   m.selectedCamera.ID,
						DeviceName: m.selectedCamera.Name,
						StreamConfig: config.StreamConfig{
							Resolution: "640x480",
							FPS:        30,
						},
					}
					config.Save(m.config)
					m.addLog("INFO", "Camera config saved")

					// Move to stepComplete
					m.cameraSetupStep = stepComplete
					m.status = "Camera setup complete!"
					m.cameraConfigured = true
					return m, nil

				//-------------------------------------------------------------------
				// stepConfigureCamera (unused in this simplified flow, but kept)
				case stepConfigureCamera:
					m.cameraConfigured = true
					m.cameraSetupStep = stepComplete
					m.status = "Camera configured!"
					return m, nil

				//-------------------------------------------------------------------
				// stepComplete -> do nothing or finalize
				case stepComplete:
					// We're done. Optionally start a final stream?
					m.addLog("INFO", "Camera is already configured (stepComplete).")
					return m, nil
				}
			}

		//-----------------------------------------------------------------------
		// 'b' to go back
		case "b", "backspace", "esc":
			if m.activeTab == cameraTab {
				switch m.cameraSetupStep {
				case stepSelectCamera:
					// Return to no config
					m.cameraSetupStep = stepNoCameraConfigured
					m.selectedCamera = camera.Device{}
					m.availableCameras = nil
					m.status = "Back to no camera selected"

				case stepTestCamera:
					// They want to go back to select a different camera.
					// Close the camera if open.
					_ = m.cameraManager.CloseCamera(m.selectedCamera.ID)
					m.cameraSetupStep = stepSelectCamera
					m.status = "Back to camera selection"

				case stepConfigureCamera:
					m.cameraSetupStep = stepTestCamera
					m.status = "Back to camera test"
				}
				return m, nil
			}

		//-----------------------------------------------------------------------
		// 's': Start/Stop server
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

		//-----------------------------------------------------------------------
		// 'p': Not implemented yet
		case "p":
			if m.activeTab == serverTab {
				// ...
			}

		//-----------------------------------------------------------------------
		// 'r': Reset camera setup
		case "r":
			if m.activeTab == cameraTab {
				// If camera is open, close it
				_ = m.cameraManager.CloseCamera(m.selectedCamera.ID)

				// Reset wizard
				m.cameraSetupStep = stepNoCameraConfigured
				m.selectedCamera = camera.Device{}
				m.availableCameras = []camera.Device{}
				m.cameraConfigured = false
				m.status = "!! Camera setup reset !!"
				return m, nil
			}

		//-----------------------------------------------------------------------
		// 'v': toggle verbosity
		case "v":
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

		//-----------------------------------------------------------------------
		// tab: cycle through tabs
		case "tab":
			m.activeTab = (m.activeTab + 1) % tabType(len(m.tabs))
		}
	}
	return m, nil
}
