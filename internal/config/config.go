package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CameraConfig holds configuration for the camera service
type StreamConfig struct {
	Resolution string `json:"resolution"`
	FPS        int    `json:"fps"`
}

type CameraConfig struct {
	DeviceName   string       `json:"device_name"`
	DeviceID     string       `json:"device_id"`
	StreamConfig StreamConfig `json:"stream_config"`
}

type DetectionConfig struct {
	ModelPath   string  `json:"model_path"`
	LabelPath   string  `json:"label_path"`
	Confidence  float32 `json:"confidence"`
	InputWidth  int     `json:"input_width"`
	InputHeight int     `json:"input_height"`
}

type AlertConfig struct {
	BirdLabel  string `json:"bird_label"`
	ToNumber   string `json:"to_number"`
	FromNumber string `json:"from_number"`
	AccountSID string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
}

type AppConfig struct {
	ServerPort      string          `json:"server_port"`
	ServerIP        string          `json:"server_ip"`
	CameraConfig    CameraConfig    `json:"camera"`
	StoragePath     string          `json:"storage_path"`
	Headless        bool            `json:"headless"`
	DetectionConfig DetectionConfig `json:"detection"`
	AlertConfig     AlertConfig     `json:"alert"`
}

// Default config
func defaultConfig() *AppConfig {
	return &AppConfig{
		CameraConfig: CameraConfig{
			DeviceName: "No Camera Configured",
			DeviceID:   "0",
			StreamConfig: StreamConfig{
				Resolution: "640x480",
				FPS:        30,
			}},
		ServerIP:    "localhost",
		ServerPort:  "8080",
		StoragePath: "",
		Headless:    false,
		DetectionConfig: DetectionConfig{
			ModelPath:   "models/detector.onnx",
			LabelPath:   "models/labels.txt",
			Confidence:  0.5,
			InputWidth:  640,
			InputHeight: 640,
		},
		AlertConfig: AlertConfig{
			BirdLabel:  "bird",
			ToNumber:   "",
			FromNumber: "",
			AccountSID: "",
			AuthToken:  "",
		},
	}
}

// getConfigPath ensures the config directory and file follow the Linux XDG convention
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine user home directory: %w", err)
	}

	// Define the path to the ~/.config/featherfinder directory
	configDir := filepath.Join(homeDir, ".config", "featherframe")

	// Ensure the directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("error creating config directory: %w", err)
	}

	// Return the full path to the config file
	return filepath.Join(configDir, "config.json"), nil
}

// Load reads the config file from the ~/.config/featherfinder directory and returns a config object
func Load() (*AppConfig, error) {
	configPath, err := getConfigPath()
	fmt.Printf("Loading config from %s\n", configPath)
	if err != nil {
		return nil, fmt.Errorf("error getting config path: %v", err)
	}

	// Check if the config file exists and return the default config if not
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaultConfig(), nil
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer configFile.Close()

	data, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Load the default config to fill in missing fields
	config := defaultConfig()

	// Unmarshal into the default config
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error unmarshalling config file: %v", err)
	}

	return config, nil
}

// Save writes the config to the ~/.config/featherfinder directory
func Save(config *AppConfig) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("error getting config path: %v", err)
	}

	// Marshal the config to JSON
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling config: %v", err)
	}

	// Write the config file
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}

	fmt.Printf("Config saved to %s\n", configPath)
	return nil
}
