package services_test

import (
	"github.com/getmentor/getmentor-api/pkg/logger"
)

func init() {
	// Initialize logger for tests
	if err := logger.Initialize(logger.Config{
		Level:       "debug",
		Environment: "development",
	}); err != nil {
		panic(err)
	}
}
