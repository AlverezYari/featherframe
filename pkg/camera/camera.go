// pkg/camera/camera.go
package camera

type Device struct {
	ID          string
	Name        string
	IsAvailable bool
	DeviceType  DeviceType
}

type DeviceType int

const (
	USBCamera DeviceType = iota
	PiCamera
	VirtualCamera
)

// Interface that platform-specific implementations will fulfill
type CameraManager interface {
	// List available cameras
	ScanDevices() ([]Device, error)

	// Open a camera for use
	OpenCamera(deviceID string) error

	// Close an open camera
	CloseCamera(deviceID string) error

	// Get a preview frame
	GetFrame(deviceID string) ([]byte, error)
}
