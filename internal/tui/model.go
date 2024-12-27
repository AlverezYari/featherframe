// internal/tui/model.go
package tui

import (
	"github.com/AlverezYari/featherframe/internal/server"
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

// Msg types
type tickMsg time.Time

// Model holds our application state
type Model struct {
	width         int
	height        int
	status        string
	isRunning     bool
	startTime     time.Time
	currentTime   time.Time
	activeTab     tabType
	tabs          []tab
	server        *server.Server
	serverPort    string
	serverRunning bool
}

// New returns a Model with initial state
func New() Model {
	now := time.Now()
	return Model{
		status:      "Starting up...",
		isRunning:   false,
		startTime:   now,
		currentTime: now,
		activeTab:   cameraTab,
		serverPort:  "8080",
		server:      server.New("8080"),
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

// Helper command for time updates
func timeTickCmd() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
