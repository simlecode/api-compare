package cmd

import "github.com/sirupsen/logrus"

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	SetLogLevel(logrus.InfoLevel)
}

func SetLogLevel(level logrus.Level) {
	logrus.SetLevel(level)
}
