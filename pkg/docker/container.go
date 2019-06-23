package docker

import (
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/term"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	// ContainerStopTimeout constant represents how long Docker Engine
	// will wait for container before stopping it
	ContainerStopTimeout = time.Millisecond * 10

	// ContainerStateRunning constants defines that container is running
	ContainerStateRunning = "running"
	// ContainerStateCreated constants defines that container is created
	ContainerStateCreated = "created"
	// ContainerStateExited constants defines that container has exited
	ContainerStateExited = "exited"
	// ContainerStateRestarting constants defines that container is restarting
	ContainerStateRestarting = "restarting"
	// ContainerStatePaused constants defines that container is paused
	ContainerStatePaused = "paused"
	// ContainerStateDead constants defines that container is dead
	ContainerStateDead = "dead"

	// ContainerArchiveDir constant represents where on container will
	// archive directory be mounted
	ContainerArchiveDir = "/archive"
	// ContainerBuildDir constant represents where on container will
	// build directory be mounted
	ContainerBuildDir = "/build"
	// ContainerSourceDir constant represents where on container will
	// source directory be mounted
	ContainerSourceDir = "/build/source"
	// ContainerCacheDir constant represents where on container will
	// cache directory be mounted
	ContainerCacheDir = "/var/cache/apt"
)

// ContainerCreateArgs struct represents arguments
// passed to ContainerCreate().
type ContainerCreateArgs struct {
	Mounts []mount.Mount
	Image  string
	Name   string
	User   string
}

// ContainerExecArgs struct represents arguments
// passed to ContainerExec().
type ContainerExecArgs struct {
	Interactive bool
	Name        string
	Cmd         string
	WorkDir     string
	AsRoot      bool
	Skip        bool
	Network     bool
}

// IsContainerCreated function checks if container is created
// or simply just exists.
func IsContainerCreated(name string) (bool, error) {
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return false, err
	}

	for i := range list {
		for j := range list[i].Names {
			if list[i].Names[j] == "/"+name {
				return true, nil
			}
		}
	}

	return false, nil
}

// IsContainerStarted function checks
// if container's state == ContainerStateRunning.
func IsContainerStarted(name string) (bool, error) {
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return false, err
	}

	for i := range list {
		for j := range list[i].Names {
			if list[i].Names[j] == "/"+name {
				if list[i].State == ContainerStateRunning {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// IsContainerStopped function checks
// if container's state != ContainerStateRunning.
func IsContainerStopped(name string) (bool, error) {
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return false, err
	}

	for i := range list {
		for j := range list[i].Names {
			if list[i].Names[j] == "/"+name {
				if list[i].State == ContainerStateRunning {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

// ContainerCreate function creates container.
//
// It's up to the caller to make to-be-mounted directories on host.
func ContainerCreate(args ContainerCreateArgs) error {
	hostConfig := &container.HostConfig{
		Mounts: args.Mounts,
	}
	config := &container.Config{
		Image: args.Image,
		User:  args.User,
	}

	_, err := cli.ContainerCreate(ctx, config, hostConfig, nil, args.Name)
	if err != nil {
		return err
	}

	return nil
}

// ContainerStart function starts container, just that.
func ContainerStart(name string) error {
	options := types.ContainerStartOptions{}

	return cli.ContainerStart(ctx, name, options)
}

// ContainerStop function stops container, just that.
//
// It utilizes ContainerStopTimeout constant.
func ContainerStop(name string) error {
	timeout := ContainerStopTimeout

	return cli.ContainerStop(ctx, name, &timeout)
}

// ContainerRemove function removes container, just that.
func ContainerRemove(name string) error {
	options := types.ContainerRemoveOptions{}

	return cli.ContainerRemove(ctx, name, options)
}

func ContainerMounts(name string) ([]mount.Mount, error) {
	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		return nil, err
	}

	mounts := make([]mount.Mount, 0)

	for _, v := range inspect.Mounts {
		mnt := mount.Mount{
			Source:   v.Source,
			Target:   v.Destination,
			Type:     v.Type,
			ReadOnly: !v.RW,
		}
		mounts = append(mounts, mnt)
	}

	return mounts, nil
}

// ContainerExec function executes a command in running container.
//
// Command is executed in bash shell by default.
//
// Command can be executed as root.
//
// Command can be executed interactively.
//
// Command can be empty, in that case just bash is executed.
func ContainerExec(args ContainerExecArgs) error {
	config := types.ExecConfig{
		Cmd:          []string{"bash"},
		WorkingDir:   args.WorkDir,
		AttachStdin:  args.Interactive,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	check := types.ExecStartCheck{
		Tty:    true,
		Detach: false,
	}

	if args.Skip {
		return nil
	}

	if args.AsRoot {
		config.User = "root"
	}

	if args.Cmd != "" {
		config.Cmd = append(config.Cmd, "-c", args.Cmd)
	}

	err := ContainerNetwork(args.Name, args.Network)
	if err != nil {
		return err
	}

	response, err := cli.ContainerExecCreate(ctx, args.Name, config)
	if err != nil {
		return err
	}

	hijack, err := cli.ContainerExecAttach(ctx, response.ID, check)
	if err != nil {
		return err
	}

	if args.Interactive {
		fd := os.Stdin.Fd()

		if term.IsTerminal(fd) {
			oldState, err := term.MakeRaw(fd)
			if err != nil {
				return err
			}
			defer term.RestoreTerminal(fd, oldState)

			err = ContainerExecResize(response.ID, fd)
			if err != nil {
				return err
			}

			go resizeIfChanged(response.ID, fd)
			go io.Copy(hijack.Conn, os.Stdin)
		}
	}

	io.Copy(os.Stdout, hijack.Conn)
	hijack.Close()

	if !args.Interactive {
		inspect, err := cli.ContainerExecInspect(ctx, response.ID)
		if err != nil {
			return err
		}

		if inspect.ExitCode != 0 {
			return errors.New("command exited with non-zero status")
		}
	}

	return nil
}

func resizeIfChanged(execID string, fd uintptr) {
	channel := make(chan os.Signal)
	signal.Notify(channel, syscall.SIGWINCH)

	for {
		<-channel
		ContainerExecResize(execID, fd)
	}
}

// ContainerExecResize function resizes TTY for exec process.
func ContainerExecResize(execID string, fd uintptr) error {
	winSize, err := term.GetWinsize(fd)
	if err != nil {
		return err
	}

	options := types.ResizeOptions{
		Height: uint(winSize.Height),
		Width:  uint(winSize.Width),
	}

	err = cli.ContainerExecResize(ctx, execID, options)
	if err != nil {
		return err
	}

	return nil
}

// ContainerNetwork checks if container is connected to network
// and then connects it or disconnects per caller request.
func ContainerNetwork(name string, wantConnected bool) error {
	network := "bridge"
	gotConnected := false

	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		return err
	}

	for net := range inspect.NetworkSettings.Networks {
		if net == network {
			gotConnected = true
		}
	}

	if wantConnected && !gotConnected {
		return cli.NetworkConnect(ctx, network, name, nil)
	}

	if !wantConnected && gotConnected {
		return cli.NetworkDisconnect(ctx, network, name, false)
	}

	return nil
}

// ContainerList returns a list of containers that match passed criteria.
func ContainerList(prefix string) ([]string, error) {
	containers := make([]string, 0)
	options := types.ContainerListOptions{
		All: true,
	}

	list, err := cli.ContainerList(ctx, options)
	if err != nil {
		return nil, err
	}

	for _, v := range list {
		for _, name := range v.Names {
			name = strings.TrimPrefix(name, "/")

			if strings.HasPrefix(name, prefix) {
				containers = append(containers, name)
			}
		}
	}

	return containers, nil
}
