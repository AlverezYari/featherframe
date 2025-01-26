// internal/logging/logging.go

package logging

import (
	"log"
	"os"
)

var Logger *log.Logger

func init() {
	file, err := os.OpenFile("featherframe.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	Logger = log.New(file, "[FeatherFinder]", log.LstdFlags|log.Lshortfile)
}
