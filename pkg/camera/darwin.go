package camera

import (
	"fmt"
	"gocv.io/x/gocv"
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

func (d *DarwinCameraManager) OpenCamera(deviceID string) error {
	// If already open, return
	if _, exists := d.openDevices[deviceID]; exists {
		return nil
	}

	// Open the camera using gocv
	cap, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		return fmt.Errorf("failed to open camera %s: %v", deviceID, err)
	}

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
