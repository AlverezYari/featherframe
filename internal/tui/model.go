// internal/tui/model.go
package tui

import (
	"fmt"
	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/server"
	"github.com/AlverezYari/featherframe/pkg/camera"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

type tabType int

const (
	cameraTab tabType = iota
	motionTab
	classificationTab
	storageTab
	serverTab
)

type tab struct {
	title string
	id    tabType
}

// Logging Setup

type Verbosity int

const (
	VerbosityError Verbosity = iota
	VerbosityInfo
	VerbosityDebug
)

type logUpdateMsg struct{}

func (m *Model) addLog(level, message string) tea.Cmd {
	logEntry := fmt.Sprintf("[%s] %s", level, message)
	m.logs = append(m.logs, logEntry)

	// Cap log buffer size
	if len(m.logs) > 1000 {
		m.logs = m.logs[1:]
	}

	// Update the log viewport content
	m.logViewport.SetContent(strings.Join(m.logs, "\n"))
	// Return a tea.Cmd to force a TUI refresh
	return func() tea.Msg {
		return logUpdateMsg{} // Define a custom message type for log updates
	}
}

func (m *Model) logCallback(level string, message string) {
	// Use m.addLog but ignore the tea.Cmd it returns
	_ = m.addLog(level, message)
}

func (m *Model) shouldShowLog(level string) bool {
	switch m.verbosity {
	case VerbosityDebug:
		return true
	case VerbosityInfo:
		return level != "DEBUG"
	case VerbosityError:
		return level == "ERROR"
	default:
		return false
	}
}

// CameraTabContent holds the content for the camera tab
type cameraSetupStep int

const (
	stepNoCameraConfigured cameraSetupStep = iota
	stepScanningForCameras
	stepSelectCamera
	stepTestCamera
	stepConfigureCamera
	stepComplete
)

type cameraMessage struct {
	text      string
	timestamp time.Time
	isError   bool
}

// Msg types
type tickMsg time.Time

// Model holds our application state
type Model struct {
	configPath       string
	config           *config.AppConfig
	width            int
	height           int
	status           string
	isRunning        bool
	startTime        time.Time
	currentTime      time.Time
	activeTab        tabType
	tabs             []tab
	server           *server.Server
	serverPort       string
	serverRunning    bool
	cameraSetupStep  cameraSetupStep
	cameraConfigured bool
	cameraManager    camera.CameraManager
	cameraMessages   []cameraMessage
	availableCameras []camera.Device
	selectedCamera   camera.Device
	logViewport      viewport.Model
	logs             []string // Log messages
	verbosity        Verbosity
}

// New returns a Model with initial state
func New(configPath string, config *config.AppConfig) Model {
	now := time.Now()

	// Create the model first
	m := Model{
		configPath:       configPath,
		config:           config,
		status:           "Starting up...",
		isRunning:        false,
		startTime:        now,
		currentTime:      now,
		activeTab:        cameraTab,
		serverPort:       config.ServerPort,
		cameraSetupStep:  stepNoCameraConfigured,
		cameraConfigured: isCameraConfigured(config.CameraConfig),
		cameraManager:    camera.NewDarwinManager(),
		availableCameras: make([]camera.Device, 0),
		tabs: []tab{
			{title: "Camera", id: cameraTab},
			{title: "Motion", id: motionTab},
			{title: "Classification", id: classificationTab},
			{title: "Storage", id: storageTab},
			{title: "Server", id: serverTab},
		},
		logViewport: func() viewport.Model {
			vp := viewport.New(0, 10)
			vp.MouseWheelEnabled = true
			vp.YPosition = 0
			return vp
		}(),
		logs: make([]string, 0),
	}

	// Initialize and start the server after creating the Model
	m.server = server.New(config.ServerPort, m.logCallback)
	err := m.server.Start()
	if err != nil {
		m.addLog("ERROR", fmt.Sprintf("Error starting server: %v", err))
	}

	// Update the status if the camera is configured
	if m.cameraConfigured {
		m.status = "Camera is configured!"
		m.cameraSetupStep = stepComplete
	} else {
		m.status = "Starting up..."
		m.cameraSetupStep = stepNoCameraConfigured
	}

	return m
}

// Init runs any initial IO
func (m Model) Init() tea.Cmd {
	return timeTickCmd()
}

// Update handles messages
func (m *Model) addCameraMessage(msg string, isError bool) {
	now := time.Now()
	if isError {
		m.status = "Error: " + msg
	}

	// Create a cameraMessage struct instead of a string
	message := cameraMessage{
		text:      msg,
		timestamp: now,
		isError:   isError,
	}

	m.cameraMessages = append(m.cameraMessages, message)
	if len(m.cameraMessages) > 10 {
		m.cameraMessages = m.cameraMessages[1:]
	}
}

// Helper function to check if a camera is configured
func isCameraConfigured(cfg config.CameraConfig) bool {
	return cfg.DeviceName != "No Camera Configured" && cfg.DeviceID != ""
}

// Helper command for time updates
func timeTickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
