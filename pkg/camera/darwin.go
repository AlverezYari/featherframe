package camera

import (
	"fmt"
	"gocv.io/x/gocv"
	"strconv"
)

type DarwinCameraManager struct {
	openDevices map[string]*gocv.VideoCapture
}

func NewDarwinManager() *DarwinCameraManager {
	return &DarwinCameraManager{
		openDevices: make(map[string]*gocv.VideoCapture),
	}
}

func (d *DarwinCameraManager) ScanDevices() ([]Device, error) {
	// On macOS, try to open cameras sequentially to find available ones
	var devices []Device

	// Usually, camera 0 is the built-in webcam
	cap, err := gocv.OpenVideoCapture(0)
	if err == nil {
		cap.Close()
		devices = append(devices, Device{
			ID:          "0",
			Name:        "Built-in Camera",
			IsAvailable: true,
			DeviceType:  USBCamera,
		})
	}

	// Check for additional cameras (up to 5)
	for i := 1; i < 5; i++ {
		cap, err := gocv.OpenVideoCapture(i)
		if err == nil {
			cap.Close()
			devices = append(devices, Device{
				ID:          fmt.Sprintf("%d", i),
				Name:        fmt.Sprintf("Camera %d", i),
				IsAvailable: true,
				DeviceType:  USBCamera,
			})
		}
	}

	return devices, nil
}

func (d *DarwinCameraManager) OpenCamera(deviceID string, config StreamConfig) error {
	// If already open, return
	if _, exists := d.openDevices[deviceID]; exists {
		return nil
	}

	// Open the camera using gocv
	var cameraIndex int
	if deviceID == "Built-in Camera" {
		cameraIndex = 0
	} else {
		var err error
		cameraIndex, err = strconv.Atoi(deviceID)
		if err != nil {
			return fmt.Errorf("invalid device ID: %s", deviceID)
		}
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

	d.openDevices[deviceID] = cap
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
	fmt.Println("GetStreamChannel called for device:", deviceID)
	cap, exists := d.openDevices[deviceID]
	if !exists {
		fmt.Println("Camera not found in openDevices map")
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	frameChan := make(chan []byte)
	go func() {
		defer close(frameChan)

		img := gocv.NewMat()
		defer img.Close()

		fmt.Println("Starting frame capture loop")
		for {
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
			fmt.Println("Sent frame to channel")

		}
	}()
	return frameChan, nil
}
