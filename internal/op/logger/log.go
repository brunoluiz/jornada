package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// New return a new logrus instance
func New(l string) *logrus.Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)

	level, err := logrus.ParseLevel(l)
	if err != nil {
		panic(err)
	}
	log.SetLevel(level)

	return log
}
