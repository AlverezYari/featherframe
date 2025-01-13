//go:build linux
// +build linux

package camera

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"gocv.io/x/gocv"
)

type LinuxCameraManager struct {
	openDevices    map[string]*gocv.VideoCapture
	doneChannels   map[string]chan struct{}
	loggerCallback func(level, message string)
}

func NewLinuxCameraManager() *LinuxCameraManager {
	return &LinuxCameraManager{
		openDevices:  make(map[string]*gocv.VideoCapture),
		doneChannels: make(map[string]chan struct{}),
	}
}

// Logging funcs
func (l *LinuxCameraManager) logMsg(level string, format string, a ...interface{}) {
	// Format the message
	msg := fmt.Sprintf(format, a...)
	// 2. Also forward to the TUI callback, if set
	if l.loggerCallback != nil {
		l.loggerCallback(level, msg)
	}
}

func (l *LinuxCameraManager) SetLoggerCallback(cb func(level, message string)) {
	l.loggerCallback = cb
}

// ScanDevices lists available cameras by invoking v4l2-ctl and parsing /dev/video* entries.
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

// listVideoDevicesLinux uses "v4l2-ctl --list-devices" to discover /dev/video* paths.
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

	l.logMsg("DEBUG", "Current set resolution: %.0fx%.0f@%.0f", actualWidth, actualHeight, actualFPS)

	l.openDevices[deviceID] = cap
	l.doneChannels[deviceID] = make(chan struct{})

	l.logMsg("INFO", "Opened camera %s successfully", deviceID)
	return nil
}

func (l *LinuxCameraManager) CloseCamera(deviceID string) error {
	cap, exists := l.openDevices[deviceID]
	if !exists {
		l.logMsg("ERROR", "Camera %s not in openDevices map", deviceID)
		return fmt.Errorf("camera %s not in openDevices map", deviceID)
	}

	if done, ok := l.doneChannels[deviceID]; ok {
		close(done)
		delete(l.doneChannels, deviceID)
	}

	if err := cap.Close(); err != nil {
		l.logMsg("ERROR", "Error closing camera %s: %v", deviceID, err)
		return fmt.Errorf("error closing camera %s: %v", deviceID, err)
	}

	delete(l.openDevices, deviceID)
	l.logMsg("INFO", "Camera %s closed successfully", deviceID)
	return nil
}

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

func (d *LinuxCameraManager) SetMode(deviceID string, mode CameraMode) error {
	// TODO: Implement actual mode switching logic
	return nil
}

func (d *LinuxCameraManager) GetMode(deviceID string) (CameraMode, error) {
	// TODO: Implement actual mode checking logic
	return ModeOff, nil
}

func (d *LinuxCameraManager) StartStream(deviceID string) error {
	// TODO: Implement actual stream starting logic
	return nil
}

func (d *LinuxCameraManager) StopStream(deviceID string) error {
	// TODO: Implement actual stream stopping logic
	return nil
}

func (d *LinuxCameraManager) IsStreaming(deviceID string) bool {
	// TODO: Implement actual streaming status check
	return false
}

func (l *LinuxCameraManager) GetStreamChannel(deviceID string) (<-chan []byte, error) {
	l.logMsg("INFO", "Starting stream for camera %s", deviceID)

	cap, exists := l.openDevices[deviceID]
	if !exists {
		l.logMsg("ERROR", "Camera %s is not open", deviceID)
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	done, ok := l.doneChannels[deviceID]
	if !ok {
		l.logMsg("ERROR", "No done channel found for camera %s", deviceID)
		return nil, fmt.Errorf("no done channel found for camera %s", deviceID)
	}

	frameChan := make(chan []byte)

	go func() {
		defer close(frameChan)

		img := gocv.NewMat()
		defer img.Close()

		l.logMsg("INFO", "Starting frame capture loop for %s", deviceID)
		for {
			select {
			case <-done:
				l.logMsg("INFO", "Stopping capture loop for %s (done channel closed)", deviceID)
				return
			default:
			}

			if !cap.IsOpened() {
				l.logMsg("ERROR", "Camera %s is no longer opened", deviceID)
				return
			}

			if ok := cap.Read(&img); !ok {
				l.logMsg("ERROR", "Failed to read frame from %s", deviceID)
				return
			}

			buf, err := gocv.IMEncode(".jpg", img)
			if err != nil {
				l.logMsg("ERROR", "Failed to encode frame from %s: %v", deviceID, err)
				continue
			}

			frameChan <- buf.GetBytes()
		}
	}()

	return frameChan, nil
}
