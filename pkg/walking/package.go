package walking

import (
	"fmt"
	"github.com/dawidd6/deber/pkg/debian"
	"github.com/dawidd6/deber/pkg/docker"
	"github.com/dawidd6/deber/pkg/log"
	"github.com/dawidd6/deber/pkg/naming"
	"github.com/dawidd6/deber/pkg/stepping"
	"os"
)

var dpkgFlags = os.Getenv("DEBER_DPKG_BUILDPACKAGE_FLAGS")

// StepPackage defines package step
var StepPackage = &stepping.Step{
	Name: "package",
	Run:  Package,
	Description: []string{
		"Runs `dpkg-buildpackage` in container.",
		"Options passed to `dpkg-buildpackage` are taken from environment variable",
		"Current `dpkg-buildpackage` options: " + dpkgFlags,
	},
}

// Package function first disables network in container,
// then executes "dpkg-buildpackage" and at the end,
// enables network back
func Package(deb *debian.Debian, dock *docker.Docker, name *naming.Naming) error {
	log.Info("Packaging software")

	// TODO duplicated code, see Update()
	file := fmt.Sprintf("%s/%s", name.ArchiveDir, "Packages")
	info, _ := os.Stat(file)
	if info == nil {
		_, err := os.Create(file)
		if err != nil {
			return log.FailE(err)
		}
	}

	err := dock.ContainerDisableNetwork(name.Container)
	if err != nil {
		return log.FailE(err)
	}
	defer dock.ContainerEnableNetwork(name.Container)

	log.Drop()

	args := docker.ContainerExecArgs{
		Name: name.Container,
		Cmd:  "dpkg-buildpackage" + " " + dpkgFlags,
	}
	err = dock.ContainerExec(args)
	if err != nil {
		return log.FailE(err)
	}

	return log.DoneE()
}
