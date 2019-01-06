// +build windows

package main

import (
	"github.com/mattn/go-colorable"
	"github.com/sirupsen/logrus"
	"runtime"
)

func init() {
	if runtime.GOOS == "windows" {
		logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
		logrus.SetOutput(colorable.NewColorableStdout())
	}
}
