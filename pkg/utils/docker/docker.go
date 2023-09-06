package docker

import (
	"context"
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// -----------------------------------------------------------------------------
// Public Functions
// -----------------------------------------------------------------------------

// InspectDockerContainer is a helper function that uses the local docker environment
// provides the full container spec for a container present in that environment by name.
func InspectDockerContainer(containerID string) (*types.ContainerJSON, error) {
	dockerc, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	containerJSON, err := dockerc.ContainerInspect(context.Background(), containerID)
	return &containerJSON, err
}

// GetDockerContainerIPNetwork supports retreiving the *net.IP4Net of a container specified
// by name (and a specified network name for the case of multiple networks).
func GetDockerContainerIPNetwork(containerID, networkName string) (*net.IPNet, error) {
	container, err := InspectDockerContainer(containerID)
	if err != nil {
		return nil, err
	}

	dockerNetwork := container.NetworkSettings.Networks[networkName]
	_, network, err := net.ParseCIDR(fmt.Sprintf("%s/%d", dockerNetwork.Gateway, dockerNetwork.IPPrefixLen))
	if err != nil {
		return nil, err
	}

	return network, nil
}
