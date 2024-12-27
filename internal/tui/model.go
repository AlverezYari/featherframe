// internal/tui/model.go
package tui

import (
	"github.com/AlverezYari/featherframe/internal/server"
	"github.com/AlverezYari/featherframe/pkg/camera"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type tabType int

const (
	cameraTab tabType = iota
	motionTab
	classificationTab
	uploadTab
	serverTab
)

type tab struct {
	title string
	id    tabType
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
	availableCameras []string
	selectedCamera   string
}

// New returns a Model with initial state
func New() Model {
	now := time.Now()
	return Model{
		status:           "Starting up...",
		isRunning:        false,
		startTime:        now,
		currentTime:      now,
		activeTab:        cameraTab,
		serverPort:       "8080",
		server:           server.New("8080"),
		cameraSetupStep:  stepNoCameraConfigured,
		cameraConfigured: false,
		cameraManager:    camera.NewDarwinManager(),
		availableCameras: make([]string, 0),
		tabs: []tab{
			{title: "Camera", id: cameraTab},
			{title: "Motion", id: motionTab},
			{title: "Classification", id: classificationTab},
			{title: "Upload", id: uploadTab},
			{title: "Server", id: serverTab},
		},
	}
}

// Init runs any initial IO
func (m Model) Init() tea.Cmd {
	return timeTickCmd()
}

// Update handles messages
func (m *Model) addCameraMessage(text string, isError bool) {
	m.cameraMessages = append(m.cameraMessages, cameraMessage{
		text:      text,
		timestamp: time.Now(),
		isError:   isError,
	})
	if len(m.cameraMessages) > 10 {
		m.cameraMessages = m.cameraMessages[1:]
	}
}

// Helper command for time updates
func timeTickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
