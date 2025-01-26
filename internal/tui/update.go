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
		m.width = msg.Width
		m.height = msg.Height
		// Re-adjust the logging viewport
		m.logViewport.Width = msg.Width - 3
		m.logViewport.Height = 10
		// Refresh logs
		m.logViewport.SetContent(strings.Join(m.logs, "\n"))
		return m, nil

	//---------------------------------------------------------------------------
	case tickMsg:
		m.currentTime = time.Time(msg)
		return m, timeTickCmd()

	//---------------------------------------------------------------------------
	case logUpdateMsg:
		// The logs were just flushed, so re-render with no side effects
		return m, nil

		//---------------------------------------------------------------------------
	// case tea.KeyMsg:
	// 	switch msg.Type {
	// 	case tea.KeyRunes:
	// 		// If we are entering storage path, append typed runes
	// 		if m.activeTab == storageTab && m.storageStep == storageStepEnterPath {
	// 			m.storagePathBuf += string(msg.Runes)
	// 		}
	// 	case tea.KeyBackspace:
	// 		if m.activeTab == storageTab && m.storageStep == storageStepEnterPath && len(m.storagePathBuf) > 0 {
	// 			// remove last character
	// 			m.storagePathBuf = m.storagePathBuf[:len(m.storagePathBuf)-1]
	// 		}
	// 	}

	case tea.KeyMsg:

		switch msg.Type {
		case tea.KeyRunes:
			// If we are entering storage path, append typed runes
			if m.activeTab == storageTab && m.storageStep == storageStepEnterPath {
				m.storagePathBuf += string(msg.Runes)
			}
		case tea.KeyBackspace:
			if m.activeTab == storageTab && m.storageStep == storageStepEnterPath && len(m.storagePathBuf) > 0 {
				// remove last character
				m.storagePathBuf = m.storagePathBuf[:len(m.storagePathBuf)-1]
			}
		}
		switch msg.String() {

		//-----------------------------------------------------------------------
		// Quit
		case "q", "ctrl+c":
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

			if m.activeTab == storageTab && m.storageStep == storageStepNone {
				m.status = "Enter storage path (press Enter to save)"
				m.storageStep = storageStepEnterPath
				m.storagePathBuf = ""
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
			if m.activeTab == storageTab && m.storageStep == storageStepEnterPath {
				if m.storagePathBuf == "" {
					m.status = "Storage path cannot be empty"
				} else {
					m.config.StoragePath = m.storagePathBuf
					if err := config.Save(m.config); err != nil {
						m.status = fmt.Sprintf("Error saving config: %v", err)
					} else {
						m.status = fmt.Sprintf("Storage path set to %s", m.storagePathBuf)
					}
					m.storageStep = storageStepConfirmed
				}
				return m, nil
			}
			if m.activeTab == cameraTab {
				m.addLog("INFO", fmt.Sprintf("Enter pressed, current step: %v", m.cameraSetupStep))

				switch m.cameraSetupStep {

				//-------------------------------------------------------------------
				// 1) stepSelectCamera -> open camera + stream -> stepTestCamera
				case stepSelectCamera:
					if m.selectedCamera.ID == "" {
						return m, nil
					}
					m.addLog("INFO", "Attempting to open camera + start stream (selectCamera)")

					// NEW: If we previously had a camera open, close it first
					// so we don't get a "file busy" error
					if m.cameraConfigured {
						m.addLog("DEBUG", "Closing any previously open camera before re-opening.")
						_ = m.cameraManager.CloseCamera(m.selectedCamera.ID)
					}

					// Open the camera
					err := m.cameraManager.OpenCamera(
						m.selectedCamera.ID,
						camera.StreamConfig{Width: 640, Height: 480, Framerate: 30},
					)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to open camera: %v", err))
						// Set user-friendly status + revert to stepSelectCamera for retry
						m.status = fmt.Sprintf("Error opening %s. Use arrow keys, press Enter to retry or 'b' to go back", m.selectedCamera.ID)
						m.cameraSetupStep = stepSelectCamera
						return m, nil
					}

					// Start the stream
					stream, err := m.cameraManager.GetStreamChannel(m.selectedCamera.ID)
					if err != nil {
						m.addLog("ERROR", fmt.Sprintf("Failed to get stream: %v", err))
						// Also revert
						m.status = fmt.Sprintf("Failed to get stream for %s. Press Enter to retry or 'b' to go back", m.selectedCamera.ID)
						m.cameraSetupStep = stepSelectCamera
						return m, nil
					}

					// Actually reading frames in background
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
				case stepConfigureCamera:
					m.cameraConfigured = true
					m.cameraSetupStep = stepComplete
					m.status = "Camera configured!"
					return m, nil

				//-------------------------------------------------------------------
				// stepComplete -> do nothing or finalize
				case stepComplete:
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
					// They want to pick a different camera, or re-try
					_ = m.cameraManager.CloseCamera(m.selectedCamera.ID)
					m.cameraSetupStep = stepSelectCamera
					m.status = "Back to camera selection"

				case stepConfigureCamera:
					m.cameraSetupStep = stepTestCamera
					m.status = "Back to camera test"
				}
				return m, nil
			}
		// On "+" or "-"
		case "+":
			if m.activeTab == motionTab {
				m.motionSensitivity += 0.1
				if m.motionSensitivity > 1.0 {
					m.motionSensitivity = 1.0
				}
				m.status = fmt.Sprintf("Motion sensitivity: %.1f", m.motionSensitivity)
			}
			return m, nil
		case "-":
			if m.activeTab == motionTab {
				m.motionSensitivity -= 0.1
				if m.motionSensitivity < 0.0 {
					m.motionSensitivity = 0.0
				}
				m.status = fmt.Sprintf("Motion sensitivity: %.1f", m.motionSensitivity)
			}
			return m, nil

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
		// 'm': Start/Stop motion detection
		case "m":
			if m.activeTab == motionTab {
				if !m.motionEnabled {
					m.status = "Motion detection ON"
					m.startMotionDetection()
				} else {
					m.status = "Motion detection OFF"
					if m.motionStopChan != nil {
						close(m.motionStopChan)
					}
				}
				return m, nil
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
