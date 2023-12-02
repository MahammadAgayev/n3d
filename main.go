package main

import (
	"n3d/cmd"

	"github.com/sirupsen/logrus"
)

func main() {
	rootCmd := cmd.NewRootCommand()

	err := rootCmd.Execute()

	if err != nil {
		logrus.Error(err)
	}
}
