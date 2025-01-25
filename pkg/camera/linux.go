package camera

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"gocv.io/x/gocv"
)

type LinuxCameraManager struct {
	openDevices      map[string]*gocv.VideoCapture
	doneChannels     map[string]chan struct{}
	streamWaitGroups map[string]*sync.WaitGroup // new: track each camera’s goroutine waitgroup

	loggerCallback func(level, message string)
}

func NewLinuxCameraManager() *LinuxCameraManager {
	return &LinuxCameraManager{
		openDevices:      make(map[string]*gocv.VideoCapture),
		doneChannels:     make(map[string]chan struct{}),
		streamWaitGroups: make(map[string]*sync.WaitGroup),
	}
}

// Logging helper
func (l *LinuxCameraManager) logMsg(level string, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	if l.loggerCallback != nil {
		l.loggerCallback(level, msg)
	}
}

func (l *LinuxCameraManager) SetLoggerCallback(cb func(level, message string)) {
	l.loggerCallback = cb
}

// List available /dev/video* devices via v4l2-ctl
func (l *LinuxCameraManager) ScanDevices() ([]Device, error) {
	names, err := listVideoDevicesLinux()
	if err != nil {
		return nil, fmt.Errorf("error scanning for cameras: %v", err)
	}

	devices := make([]Device, 0, len(names))
	for _, path := range names {
		devices = append(devices, Device{
			ID:          path,
			Name:        fmt.Sprintf("Camera at %s", path),
			IsAvailable: true,
			DeviceType:  USBCamera,
		})
	}
	return devices, nil
}

// Runs `v4l2-ctl --list-devices` to find lines with /dev/video\d+
func listVideoDevicesLinux() ([]string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("v4l2-ctl", "--list-devices")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("couldn't list devices: %w\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()
	re := regexp.MustCompile(`/dev/video\d+`)
	lines := strings.Split(output, "\n")

	var devices []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if re.MatchString(line) {
			devices = append(devices, line)
		}
	}

	// Make unique
	unique := make(map[string]struct{})
	for _, dev := range devices {
		unique[dev] = struct{}{}
	}

	devices = devices[:0]
	for dev := range unique {
		devices = append(devices, dev)
	}
	return devices, nil
}

// OpenCamera opens the camera by path (like /dev/video0) and sets resolution/fps
func (l *LinuxCameraManager) OpenCamera(deviceID string, cfg StreamConfig) error {
	l.logMsg("DEBUG", "Attempting to open camera deviceID=%q", deviceID)
	cap, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		l.logMsg("ERROR", "Cannot open camera %s: %v", deviceID, err)
		return fmt.Errorf("cannot open camera %s: %v", deviceID, err)
	}

	cap.Set(gocv.VideoCaptureFrameWidth, float64(cfg.Width))
	cap.Set(gocv.VideoCaptureFrameHeight, float64(cfg.Height))
	cap.Set(gocv.VideoCaptureFPS, float64(cfg.Framerate))

	actualWidth := cap.Get(gocv.VideoCaptureFrameWidth)
	actualHeight := cap.Get(gocv.VideoCaptureFrameHeight)
	actualFPS := cap.Get(gocv.VideoCaptureFPS)

	l.logMsg("DEBUG", "Current set resolution: %.0fx%.0f@%.0f",
		actualWidth, actualHeight, actualFPS)

	// Store references
	l.openDevices[deviceID] = cap
	l.doneChannels[deviceID] = make(chan struct{})

	// If there's any old waitgroup from a prior run, remove it.
	delete(l.streamWaitGroups, deviceID)

	l.logMsg("INFO", "Opened camera %s successfully", deviceID)
	return nil
}

// CloseCamera signals the streaming goroutine to end, waits for it, then closes the device
func (l *LinuxCameraManager) CloseCamera(deviceID string) error {
	// 1) confirm device is in openDevices
	cap, exists := l.openDevices[deviceID]
	if !exists {
		l.logMsg("ERROR", "CloseCamera: camera %s not in openDevices map", deviceID)
		return fmt.Errorf("camera %s not in openDevices map", deviceID)
	}

	// 2) close the done channel so the goroutine reading frames can exit
	done, ok := l.doneChannels[deviceID]
	if ok {
		close(done)
		delete(l.doneChannels, deviceID)
	}

	// 3) wait for the goroutine to finish if it’s currently streaming
	wg, wgOk := l.streamWaitGroups[deviceID]
	if wgOk {
		l.logMsg("DEBUG", "CloseCamera: waiting for capture goroutine to finish for %s", deviceID)
		wg.Wait()
		// remove it from map
		delete(l.streamWaitGroups, deviceID)
	}

	// 4) now safe to close the capture
	if err := cap.Close(); err != nil {
		l.logMsg("ERROR", "Error closing camera %s: %v", deviceID, err)
		return fmt.Errorf("error closing camera %s: %v", deviceID, err)
	}

	// 5) remove from openDevices
	delete(l.openDevices, deviceID)
	l.logMsg("INFO", "Camera %s closed successfully", deviceID)
	return nil
}

// GetFrame does a single read/encode (synchronous)
func (l *LinuxCameraManager) GetFrame(deviceID string) ([]byte, error) {
	cap, exists := l.openDevices[deviceID]
	if !exists {
		l.logMsg("ERROR", "Camera %s is not open", deviceID)
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	img := gocv.NewMat()
	defer img.Close()

	if ok := cap.Read(&img); !ok {
		l.logMsg("ERROR", "Failed to read frame from camera %s", deviceID)
		return nil, fmt.Errorf("failed to read frame from camera %s", deviceID)
	}

	buf, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		l.logMsg("ERROR", "Failed to encode frame: %v", err)
		return nil, fmt.Errorf("failed to encode frame: %v", err)
	}
	return buf.GetBytes(), nil
}

// GetStreamChannel spawns a goroutine reading frames until done-ch is closed
func (l *LinuxCameraManager) GetStreamChannel(deviceID string) (<-chan []byte, error) {
	l.logMsg("INFO", "Starting stream for camera %s", deviceID)

	cap, exists := l.openDevices[deviceID]
	if !exists {
		l.logMsg("WARN", "Camera %s is not open - returning empty channel", deviceID)
		empty := make(chan []byte)
		close(empty)
		return empty, nil
	}

	done, ok := l.doneChannels[deviceID]
	if !ok {
		l.logMsg("ERROR", "No done channel found for camera %s", deviceID)
		empty := make(chan []byte)
		close(empty)
		return empty, fmt.Errorf("no done channel found for camera %s", deviceID)
	}
	// Make a channel to push frames
	frameChan := make(chan []byte, 5)

	// Create or reuse a WaitGroup for this camera
	wg := &sync.WaitGroup{}
	l.streamWaitGroups[deviceID] = wg

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(frameChan)

		img := gocv.NewMat()
		defer img.Close()

		l.logMsg("INFO", "Starting frame capture loop for %s", deviceID)

		// Simple fps-limiter
		fps := 30
		frameDuration := time.Second / time.Duration(fps)

		for {
			start := time.Now()

			select {
			case <-done:
				l.logMsg("INFO", "Stopping capture loop for %s (done channel closed)", deviceID)
				return
			default:
				// proceed
			}

			// Check if device is still open
			if !cap.IsOpened() {
				l.logMsg("ERROR", "Camera %s is no longer opened", deviceID)
				return
			}

			// Read frame
			if ok := cap.Read(&img); !ok {
				l.logMsg("ERROR", "Failed to read frame from %s", deviceID)
				return
			}

			// Encode as JPEG
			buf, err := gocv.IMEncode(".jpg", img)
			if err != nil {
				l.logMsg("ERROR", "Failed to encode frame from %s: %v", deviceID, err)
				continue
			}
			bufBytes := buf.GetBytes()
			buf.Close()

			// Send or drop if channel is full
			select {
			case frameChan <- bufBytes:
			default:
				l.logMsg("DEBUG", "Dropping a frame for %s because the channel is full", deviceID)
			}

			// Enforce ~30 fps
			elapsed := time.Since(start)
			if elapsed < frameDuration {
				time.Sleep(frameDuration - elapsed)
			}
		}
	}()

	return frameChan, nil
}

// The rest: stubs for Mode, Start/StopStream, etc.
func (l *LinuxCameraManager) SetMode(deviceID string, mode CameraMode) error {
	return nil
}
func (l *LinuxCameraManager) GetMode(deviceID string) (CameraMode, error) {
	return ModeOff, nil
}
func (l *LinuxCameraManager) StartStream(deviceID string) error {
	return nil
}
func (l *LinuxCameraManager) StopStream(deviceID string) error {
	return nil
}
func (l *LinuxCameraManager) IsStreaming(deviceID string) bool {
	return false
}
