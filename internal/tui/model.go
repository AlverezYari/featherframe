package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/server"
	"github.com/AlverezYari/featherframe/pkg/camera"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Tabs
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

// Verbosity
type Verbosity int

const (
	VerbosityError Verbosity = iota
	VerbosityInfo
	VerbosityDebug
)

// We’ll use a custom message for log updates
type logUpdateMsg struct{}

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

type tickMsg time.Time

// The main Model (use pointer receivers for stateful changes)
type Model struct {
	// Config / Basic Fields
	configPath  string
	config      *config.AppConfig
	width       int
	height      int
	status      string
	isRunning   bool
	startTime   time.Time
	currentTime time.Time

	// Tabs
	activeTab tabType
	tabs      []tab

	// Server
	server        *server.Server
	serverPort    string
	serverRunning bool

	// Camera
	cameraSetupStep  cameraSetupStep
	cameraConfigured bool
	cameraManager    camera.CameraManager
	cameraMessages   []cameraMessage
	availableCameras []camera.Device
	selectedCamera   camera.Device

	// Logging / Verbosity
	logViewport   viewport.Model
	logs          []string // Lines actually displayed
	verbosity     Verbosity
	logBuffer     []string      // Temporary buffer for new log lines
	lastLogUpdate time.Time     // When logs were last flushed
	logThrottle   time.Duration // Wait time between re-renders
}

// New returns a pointer to a Model with initial state
func New(configPath string, cfg *config.AppConfig) *Model {
	now := time.Now()

	m := &Model{
		configPath:       configPath,
		config:           cfg,
		status:           "Starting up...",
		startTime:        now,
		currentTime:      now,
		activeTab:        cameraTab,
		serverPort:       cfg.ServerPort,
		cameraSetupStep:  stepNoCameraConfigured,
		cameraConfigured: isCameraConfigured(cfg.CameraConfig),
		cameraManager:    camera.NewDarwinManager(),
		availableCameras: []camera.Device{},
		tabs: []tab{
			{title: "Camera", id: cameraTab},
			{title: "Motion", id: motionTab},
			{title: "Classification", id: classificationTab},
			{title: "Storage", id: storageTab},
			{title: "Server", id: serverTab},
		},
		// Logging
		logs:          make([]string, 0),
		logBuffer:     make([]string, 0),
		lastLogUpdate: now,
		logThrottle:   200 * time.Millisecond, // Adjust as needed
	}

	// Set the logging verbosity
	m.verbosity = VerbosityInfo

	// Init the log viewport
	vp := viewport.New(0, 10)
	vp.MouseWheelEnabled = true
	vp.Width = 80 // updated in Update if window resizes
	vp.Height = 10
	vp.YPosition = 0
	vp.SetContent("")
	m.logViewport = vp

	// Start the server
	m.server = server.New(cfg.ServerPort, m.logCallback)
	if err := m.server.Start(); err != nil {
		// Force a log so we see it immediately
		m.flushLogImmediately("ERROR", fmt.Sprintf("Error starting server: %v", err))
	}

	// If camera is configured, update status
	if m.cameraConfigured {
		m.status = "Camera is configured!"
		m.cameraSetupStep = stepComplete
	} else {
		m.status = "Starting up..."
		m.cameraSetupStep = stepNoCameraConfigured
	}
	return m
}

// Init is part of Bubble Tea’s Model interface
func (m *Model) Init() tea.Cmd {
	return timeTickCmd()
}

func timeTickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- Logging logic ---

// Called by the server for each log line
func (m *Model) logCallback(level string, message string) {
	if m.shouldShowLog(level) {
		_ = m.addLog(level, message) // ignore the returned Cmd; we rely on throttling
	}
}

func (m *Model) shouldShowLog(level string) bool {
	switch m.verbosity {
	case VerbosityDebug:
		return true // show all logs
	case VerbosityInfo:
		return level != "DEBUG" // show everything except debug
	case VerbosityError:
		return level == "ERROR" // show only errors
	}
	return false
}

// addLog buffers logs, flushes if enough time elapsed
func (m *Model) addLog(level, message string) tea.Cmd {
	entry := fmt.Sprintf("[%s] %s", level, message)
	m.logBuffer = append(m.logBuffer, entry)

	now := time.Now()
	if now.Sub(m.lastLogUpdate) >= m.logThrottle {
		return m.flushLogs(now)
	}
	return nil
}

// flushLogs moves buffered lines to m.logs, sets viewport content, returns Cmd to re-render
func (m *Model) flushLogs(flushTime time.Time) tea.Cmd {
	m.logs = append(m.logs, m.logBuffer...)
	m.logBuffer = m.logBuffer[:0] // clear buffer
	m.lastLogUpdate = flushTime

	// Limit total lines
	if len(m.logs) > 300 {
		m.logs = m.logs[len(m.logs)-300:]
	}

	m.logViewport.SetContent(strings.Join(m.logs, "\n"))
	return func() tea.Msg {
		return logUpdateMsg{}
	}
}

// flushLogImmediately is used if we need an immediate log (e.g. server start error)
func (m *Model) flushLogImmediately(level, message string) {
	entry := fmt.Sprintf("[%s] %s", level, message)
	m.logs = append(m.logs, entry)
	// Also update viewport now
	if len(m.logs) > 300 {
		m.logs = m.logs[len(m.logs)-300:]
	}
	m.logViewport.SetContent(strings.Join(m.logs, "\n"))
}

// Helper function to check if a camera is configured
func isCameraConfigured(cfg config.CameraConfig) bool {
	return cfg.DeviceName != "No Camera Configured" && cfg.DeviceID != ""
}

// Add a cameraMessage
func (m *Model) addCameraMessage(msg string, isError bool) {
	if isError {
		m.status = "Error: " + msg
	}
	cm := cameraMessage{
		text:      msg,
		timestamp: time.Now(),
		isError:   isError,
	}
	m.cameraMessages = append(m.cameraMessages, cm)
	if len(m.cameraMessages) > 10 {
		m.cameraMessages = m.cameraMessages[1:]
	}
}
