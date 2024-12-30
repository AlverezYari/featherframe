package camera

import (
	"bytes"
	"fmt"
	"gocv.io/x/gocv"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type DarwinCameraManager struct {
	openDevices map[string]*gocv.VideoCapture
	logger      *log.Logger
}

func NewDarwinManager() *DarwinCameraManager {
	return &DarwinCameraManager{
		openDevices: make(map[string]*gocv.VideoCapture),
		logger:      log.New(os.Stdout, "[CAMERA] ", log.LstdFlags),
	}
}

func (d *DarwinCameraManager) ScanDevices() ([]Device, error) {
	// Call ListVideoDevices to get device names
	names, err := ListVideoDevices()
	if err != nil {
		return nil, err
	}

	// Convert device names into Device objects
	devices := []Device{}
	for i, name := range names {
		devices = append(devices, Device{
			ID:          fmt.Sprintf("%d", i),
			Name:        name,
			IsAvailable: true,
			DeviceType:  USBCamera,
		})
	}

	return devices, nil
}

// ListVideoDevices retrieves up to a maximum of 10 video device names using ffmpeg.

func ListVideoDevices() ([]string, error) {
	// Buffer to capture FFmpeg's stderr output
	var stderr bytes.Buffer

	// Run FFmpeg command to list devices
	cmd := exec.Command("ffmpeg", "-f", "avfoundation", "-list_devices", "true", "-i", "")
	cmd.Stderr = &stderr // Capture stderr internally

	// Run the command
	err := cmd.Run()

	// Capture FFmpeg output for parsing
	output := stderr.String()

	// If the command failed, check if it produced valid output
	if err != nil {
		// Check if the output contains the device list section
		if strings.Contains(output, "AVFoundation video devices:") {
			// Non-critical error; continue parsing
		} else {
			// Critical error; return it
			return nil, fmt.Errorf("error running ffmpeg command: %w", err)
		}
	}

	// Parse the output for video devices
	videoDevices := []string{}
	re := regexp.MustCompile(`(?m)^\[AVFoundation indev .*?\] \[(\d+)\] (.+)$`)
	inVideoSection := false
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "AVFoundation video devices:") {
			inVideoSection = true
			continue
		}
		if strings.Contains(line, "AVFoundation audio devices:") {
			break
		}

		if inVideoSection {
			match := re.FindStringSubmatch(line)
			if len(match) > 2 {
				videoDevices = append(videoDevices, match[2])
			}
		}
	}

	return videoDevices, nil
}

func (d *DarwinCameraManager) OpenCamera(deviceID string, config StreamConfig) error {
	// If already open, return
	if _, exists := d.openDevices[deviceID]; exists {
		return nil
	}

	// Open the camera using gocv
	var cameraIndex int
	cameraIndex, err := strconv.Atoi(deviceID)
	if err != nil {
		return fmt.Errorf("invalid device ID: %s", deviceID)
	}

	cap, err := gocv.OpenVideoCapture(cameraIndex)
	if err != nil {
		return fmt.Errorf("error opening camera %s: %v", deviceID, err)
	}

	if !cap.IsOpened() {
		cap.Close()
		return fmt.Errorf("camera %s is not open", deviceID)
	}

	// Apply configuration
	cap.Set(gocv.VideoCaptureFrameWidth, float64(config.Width))

	cap.Set(gocv.VideoCaptureFrameHeight, float64(config.Height))

	cap.Set(gocv.VideoCaptureFPS, float64(config.Framerate))

	fmt.Printf("About to store camera with ID: %s\n", deviceID)
	d.openDevices[deviceID] = cap
	fmt.Printf("Stored camera. Current open devices: %v\n", d.openDevices)

	return nil
}

func (d *DarwinCameraManager) CloseCamera(deviceID string) error {
	if cap, exists := d.openDevices[deviceID]; exists {
		if err := cap.Close(); err != nil {
			return fmt.Errorf("error closing camera %s: %v", deviceID, err)
		}
		delete(d.openDevices, deviceID)
	}
	return nil
}

func (d *DarwinCameraManager) GetFrame(deviceID string) ([]byte, error) {
	cap, exists := d.openDevices[deviceID]
	if !exists {
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	img := gocv.NewMat()
	defer img.Close()

	if ok := cap.Read(&img); !ok {
		return nil, fmt.Errorf("failed to read frame from camera %s", deviceID)
	}

	// Convert to JPG for preview
	buf, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode frame: %v", err)
	}

	return buf.GetBytes(), nil

}

func (d *DarwinCameraManager) SetMode(deviceID string, mode CameraMode) error {
	// TODO: Implement actual mode switching logic
	return nil
}

func (d *DarwinCameraManager) GetMode(deviceID string) (CameraMode, error) {
	// TODO: Implement actual mode checking logic
	return ModeOff, nil
}

func (d *DarwinCameraManager) StartStream(deviceID string) error {
	// TODO: Implement actual stream starting logic
	return nil
}

func (d *DarwinCameraManager) StopStream(deviceID string) error {
	// TODO: Implement actual stream stopping logic
	return nil
}

func (d *DarwinCameraManager) IsStreaming(deviceID string) bool {
	// TODO: Implement actual streaming status check
	return false
}

func (d *DarwinCameraManager) GetStreamChannel(deviceID string) (<-chan []byte, error) {
	d.logger.Printf("Starting stream for camera %s", deviceID)
	cap, exists := d.openDevices[deviceID]
	if !exists {
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	frameChan := make(chan []byte)
	go func() {
		defer close(frameChan)

		img := gocv.NewMat()
		defer img.Close()

		fmt.Println("Starting frame capture loop")
		for {
			if !cap.IsOpened() {
				d.logger.Printf("Camera %s closed unexpectedly", deviceID)
				return
			}
			if ok := cap.Read(&img); !ok {
				fmt.Println("Failed to read frame")
				return
			}

			// Convert to JPG for streaming
			buf, err := gocv.IMEncode(".jpg", img)
			if err != nil {
				fmt.Println("Failed to encode frame:", err)
				continue
			}

			frameChan <- buf.GetBytes()

		}
	}()
	return frameChan, nil
}
