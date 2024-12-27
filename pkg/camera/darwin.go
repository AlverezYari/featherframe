package camera

import (
	"fmt"
	"gocv.io/x/gocv"
	"time"
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
	cap, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		return fmt.Errorf("failed to open camera %s: %v", deviceID, err)
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
	cap, exists := d.openDevices[deviceID]
	if !exists {
		return nil, fmt.Errorf("camera %s is not open", deviceID)
	}

	frameChan := make(chan []byte)
	go func() {
		defer close(frameChan)

		img := gocv.NewMat()
		defer img.Close()

		for {
			if ok := cap.Read(&img); !ok {
				return
			}

			// Convert to JPG for streaming
			buf, err := gocv.IMEncode(".jpg", img)
			if err != nil {
				continue
			}

			frameChan <- buf.GetBytes()
			time.Sleep(time.Second / 30) // 30 FPS
		}
	}()
	return frameChan, nil
}
