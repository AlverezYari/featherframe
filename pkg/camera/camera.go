// pkg/camera/camera.go
package camera

type DeviceType int

const (
	USBCamera DeviceType = iota
	PiCamera
	VirtualCamera
)

type Device struct {
	ID          string
	Name        string
	IsAvailable bool
	DeviceType  DeviceType
}

type CameraMode int

const (
	ModeOff         CameraMode = iota
	ModeStreaming              // Basic streaming, no UI
	ModeLiveMonitor            // Streaming + web UI for live viewing
	ModeFocus                  // Streaming + web UI with focus tools
	ModeAutoMonitor            // Streaming + motion detection, no UI required
)

type StreamConfig struct {
	Width     int
	Height    int
	Framerate int
	Mode      CameraMode
	AutoStart bool
}

type CameraManager interface {
	// Discovery and basic control
	ScanDevices() ([]Device, error)
	OpenCamera(deviceID string, config StreamConfig) error
	CloseCamera(deviceID string) error

	// Mode and stream control
	SetMode(deviceID string, mode CameraMode) error
	GetMode(deviceID string) (CameraMode, error)
	StartStream(deviceID string) error
	StopStream(deviceID string) error
	IsStreaming(deviceID string) bool

	// Stream access
	GetStreamChannel(deviceID string) (<-chan []byte, error)
}
