package logger

import "github.com/sirupsen/logrus"

var Log = logrus.New()

func Initialize(level string) error {
	var err error
	logLvl := logrus.InfoLevel
	if level != "" {
		logLvl, err = logrus.ParseLevel(level)
		if err != nil {
			return err
		}
	}
	Log.SetLevel(logLvl)
	Log.SetFormatter(&logrus.JSONFormatter{})
	return nil
}
