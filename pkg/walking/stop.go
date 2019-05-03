package walking

import (
	"github.com/dawidd6/deber/pkg/debian"
	"github.com/dawidd6/deber/pkg/docker"
	"github.com/dawidd6/deber/pkg/log"
	"github.com/dawidd6/deber/pkg/naming"
	"github.com/dawidd6/deber/pkg/stepping"
)

// StepStop defines stop step
var StepStop = &stepping.Step{
	Name: "stop",
	Run:  Stop,
	Description: []string{
		"Stops container.",
		"With " + docker.ContainerStopTimeout.String() + " timeout.",
	},
}

// Stop function commands Docker Engine to stop container
func Stop(deb *debian.Debian, dock *docker.Docker, name *naming.Naming) error {
	log.Info("Stopping container")

	isContainerStopped, err := dock.IsContainerStopped(name.Container)
	if err != nil {
		return log.FailE(err)
	}
	if isContainerStopped {
		return log.SkipE()
	}

	err = dock.ContainerStop(name.Container)
	if err != nil {
		return log.FailE(err)
	}

	return log.DoneE()
}
