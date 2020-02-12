package handler

import (
	"github.com/sirupsen/logrus"
)

var (
	log *logrus.Entry
)

func init() {
	log = logrus.WithFields(logrus.Fields{"component": "http_handler"})

}
