// Package docker wraps Docker Go SDK for internal usage in deber.
package docker

import (
	"context"
	"github.com/docker/docker/client"
)

const (
	// APIVersion constant is the minimum supported version of Docker Engine API
	APIVersion = "1.30"
)

var (
	cli *client.Client
	ctx = context.Background()
)

// New function creates fresh Docker struct and connects to Docker Engine.
func New() error {
	c, err := client.NewClientWithOpts(client.WithVersion(APIVersion))
	if err != nil {
		return err
	}

	cli = c

	return nil
}
