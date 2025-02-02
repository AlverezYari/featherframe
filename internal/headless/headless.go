package headless

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AlverezYari/featherframe/internal/config"
	"github.com/AlverezYari/featherframe/internal/logging"
	"github.com/AlverezYari/featherframe/internal/server"
	"github.com/AlverezYari/featherframe/pkg/camera"
)

func NewHeadless(configPath string, cfg *config.AppConfig) error {

	logger := logging.Logger

	logger.Println(fmt.Sprintln("headless is running.."))
	logger.Println(time.Now().String())

	// Added check for different goos here.. starting with linux to get it running
	camMgr := camera.NewLinuxCameraManager()
	// ! - TODO: add error handling here

	// Set our logging callback func to forward over stuff, might need to reorg this to be normal logger first, then -> to file -> the to bubble tea's logger.

	camMgr.SetLoggerCallback(func(level, msg string) {

		fmt.Printf("[%s] %s\n", level, msg)
	})

	// Check to see if we have camera setup
	if cfg.CameraConfig.DeviceID == "" {
		fmt.Println("No camera configured, cannot run without a camera configured, exiting..")
		return fmt.Errorf("no camera configured!")
	}

	fmt.Println("===== Config Info: =====")
	fmt.Println(configPath, cfg)
	fmt.Println("========================")

	// Parse our our camera config; resoution, fps, etc.
	width, height := parseResolution(cfg.CameraConfig.StreamConfig.Resolution)
	fps := float64(cfg.CameraConfig.StreamConfig.FPS)
	opencfg := camera.StreamConfig{
		Width:     width,
		Height:    height,
		Framerate: int(fps),
		Mode:      camera.ModeLiveMonitor,
		AutoStart: true,
	}

	// Start our webserver
	srv := server.New(
		cfg.ServerPort,
		func(level, message string) {
			// simple logging callback
			fmt.Printf("[SERVER %s] %s\n", level, message)
		},
		camMgr,
		cfg.CameraConfig.DeviceID,
		cfg,
	)
	if err := srv.Start(); err != nil {
		return fmt.Errorf("headless: failed to start the webserver: %w", err)
	}

	// Attempt to open camera with our config
	if err := camMgr.OpenCamera(cfg.CameraConfig.DeviceID, opencfg); err != nil {
		return fmt.Errorf("headless: failed to open camera: %v", err)
	}

	// Test our camera's functionality by reading a single frame
	frame, err := camMgr.GetFrame(cfg.CameraConfig.DeviceID)
	if err != nil {
		return fmt.Errorf("headless: failed to read test frame: %v", err)
	}
	fmt.Printf("Successfully read a test frame (%d bytes)\n", len(frame))

	// Start our streaming to live monitor
	streamCh, err := camMgr.GetStreamChannel(cfg.CameraConfig.DeviceID, cfg.CameraConfig.StreamConfig.FPS)
	if err != nil {
		return fmt.Errorf("Failed to start stream channel: %v", err)
	}
	go func() {
		for frm := range streamCh {
			srv.BroadcastFrame(frm)
		}
		fmt.Printf("[HEADLESS] stream ended (camera closed).")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-c:
		fmt.Println("Received shutdown signal. Closing camera & server...")
		_ = camMgr.CloseCamera(cfg.CameraConfig.DeviceID)
		_ = srv.Stop()
		return nil
	}

}

func parseResolution(res string) (int, int) {
	var w, h int
	fmt.Sscanf(res, "%dx%d", &w, &h)
	if w <= 0 || h <= 0 {
		w, h = 640, 480 // fallback for dev
	}
	return w, h
}
