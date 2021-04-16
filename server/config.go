package server

import (
	"github.com/sirupsen/logrus"
)

// Config bundles configuration settings.
type Config struct {
	ListenAddress string

	Logger logrus.FieldLogger
}
