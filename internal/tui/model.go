package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/server"
	"github.com/AlverezYari/featherframe/pkg/camera"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gocv.io/x/gocv"

	"os"
	"path/filepath"
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

// Storage

type storageSetupStep int

const (
	storageStepNone storageSetupStep = iota
	storageStepEnterPath
	storageStepConfirmed
)

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
	isStreaming      bool

	// Motion
	motionEnabled     bool
	motionSensitivity float64
	motionStopChan    chan struct{}
	motionCooldown    time.Duration
	lastCaptureTime   time.Time
	// Storage
	storageStep    storageSetupStep
	storagePathBuf string // Buffer for user input

	// Logging / Verbosity
	logViewport   viewport.Model
	logs          []string // Lines actually displayed
	verbosity     Verbosity
	logBuffer     []string      // Temporary buffer for new log lines
	lastLogUpdate time.Time     // When logs were last flushed
	logThrottle   time.Duration // Wait time between re-renders
}

func newCameraManager(logCallback func(level, message string)) camera.CameraManager {
	switch runtime.GOOS {
	case "darwin":
		mgr := camera.NewDarwinManager()
		mgr.SetLoggerCallback(logCallback)
		return mgr
	case "linux":
		mgr := camera.NewLinuxCameraManager()
		mgr.SetLoggerCallback(logCallback)
		return mgr
	default:
		mgr := camera.NewDarwinManager()
		mgr.SetLoggerCallback(logCallback)
		return mgr
	}
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
		// cameraManager: ZeroVal, we'll set it in directly after our model is invoked.
		availableCameras: []camera.Device{},
		tabs: []tab{
			{title: "Camera", id: cameraTab},
			{title: "Motion", id: motionTab},
			{title: "Classification", id: classificationTab},
			{title: "Storage", id: storageTab},
			{title: "Server", id: serverTab},
		},
		logs:          make([]string, 0),
		logBuffer:     make([]string, 0),
		lastLogUpdate: now,
		logThrottle:   200 * time.Millisecond, // adjust as needed
	}

	// Create our cameraManager
	cm := newCameraManager(m.logCallback)
	m.cameraManager = cm

	// Set our motion defaults
	m.motionEnabled = false
	m.motionSensitivity = 0.2
	m.motionStopChan = nil
	m.motionCooldown = 3 * time.Second
	m.lastCaptureTime = time.Time{}

	// Set our Storage
	m.storageStep = storageStepNone
	m.storagePathBuf = ""

	m.verbosity = VerbosityInfo

	vp := viewport.New(0, 10)
	vp.MouseWheelEnabled = true
	vp.Width = 80
	vp.Height = 10
	vp.YPosition = 0
	vp.SetContent("")
	m.logViewport = vp

	// Start the server
	m.server = server.New(cfg.ServerPort, m.logCallback, m.cameraManager, cfg.CameraConfig.DeviceID, cfg)
	if err := m.server.Start(); err != nil {
		m.flushLogImmediately("ERROR", fmt.Sprintf("Error starting server: %v", err))
	}

	if m.cameraConfigured {
		m.status = "Camera is configured!"
		m.cameraSetupStep = stepComplete

		// OPEN the camera right now, start streaming for live-monitor
		err := m.cameraManager.OpenCamera(
			cfg.CameraConfig.DeviceID,
			camera.StreamConfig{
				Width: 640, Height: 480, Framerate: 30,
			},
		)
		if err != nil {
			m.flushLogImmediately("ERROR",
				fmt.Sprintf("Error opening camera on startup: %v", err))
		} else {
			ch, err2 := m.cameraManager.GetStreamChannel(cfg.CameraConfig.DeviceID)
			if err2 != nil {
				m.flushLogImmediately("ERROR",
					fmt.Sprintf("Error starting stream on startup: %v", err2))
			} else {
				go func() {
					for frame := range ch {
						m.server.BroadcastFrame(frame)
					}
				}()
			}
		}
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

// Motion Dection helper function
func (m *Model) startMotionDetection() {
	if m.motionEnabled {
		return
	}
	m.motionStopChan = make(chan struct{})
	m.motionEnabled = true

	go func() {
		defer func() { m.motionEnabled = false }()

		devID := m.config.CameraConfig.DeviceID
		frameCh, err := m.cameraManager.GetStreamChannel(devID)
		if err != nil {
			m.addLog("ERROR", fmt.Sprintf("Motion detect stream error: %v", err))
			return
		}

		var prevFrame []byte

		for {
			select {
			case <-m.motionStopChan:
				m.addLog("INFO", "Motion detection stopped.")
				return

			case frame, ok := <-frameCh:
				if !ok {
					m.addLog("INFO", "Motion detection stream ended.")
					return
				}

				// Compare "frame" to "prevFrame"
				if len(prevFrame) > 0 {
					if motionDetected(prevFrame, frame, m.motionSensitivity) {
						// Check cooldown
						now := time.Now()
						if now.Sub(m.lastCaptureTime) >= m.motionCooldown {
							// Enough time since last capture, so take a screenshot
							err := m.takeScreenshot()
							if err != nil {
								m.addLog("ERROR", fmt.Sprintf("Failed to take screenshot: %v", err))
							} else {
								m.addLog("INFO", "Motion detected, screenshot taken!")
								m.lastCaptureTime = now
							}
						} else {
							// Still within cooldown; skip
							// (Optional) Log or ignore
							// m.addLog("DEBUG", "Motion detected but still cooling down")
						}
					}
				}
				prevFrame = frame
			}
		}
	}()
}

// A naive motionDetected function
func motionDetected(prevFrame, curFrame []byte, sensitivity float64) bool {
	matPrev, err1 := gocv.IMDecode(prevFrame, gocv.IMReadColor)
	matCurr, err2 := gocv.IMDecode(curFrame, gocv.IMReadColor)
	if err1 != nil || err2 != nil || matPrev.Empty() || matCurr.Empty() {
		return false
	}
	defer matPrev.Close()
	defer matCurr.Close()

	diff := gocv.NewMat()
	defer diff.Close()
	gocv.AbsDiff(matPrev, matCurr, &diff)

	sumVals := diff.Sum()
	totalDiff := sumVals.Val1 + sumVals.Val2 + sumVals.Val3 // B+G+R

	threshold := 50000.0 * (1.0 - sensitivity) // TOTALLY arbitrary
	return totalDiff > threshold
}

// takeScreenshot
func (m *Model) takeScreenshot() error {
	devID := m.config.CameraConfig.DeviceID
	frame, err := m.cameraManager.GetFrame(devID)
	if err != nil {
		return err
	}
	storePath := m.config.StoragePath
	if storePath == "" {
		m.addLog("WARN", "No storage path set. Skipping save.")
		return nil
	}
	filename := fmt.Sprintf("motion_%s.jpg", time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(storePath, filename)
	return os.WriteFile(fullPath, frame, 0644)
}
