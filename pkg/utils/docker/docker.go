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
	ctx := context.Background()
	dockerc, err := NewNegotiatedClientWithOpts(ctx, client.FromEnv)
	if err != nil {
		return nil, err
	}
	containerJSON, err := dockerc.ContainerInspect(ctx, containerID)
	return &containerJSON, err
}

// TODO should be converted to net/ip to net/netip, but this requires a breaking change to a public function

// GetDockerContainerIPNetwork supports retreiving the *net.IP4Net of a container specified
// by name (and a specified network name for the case of multiple networks).
func GetDockerContainerIPNetwork(containerID, networkName string) (*net.IPNet, *net.IPNet, error) {
	container, err := InspectDockerContainer(containerID)
	if err != nil {
		return nil, nil, err
	}

	dockerNetwork := container.NetworkSettings.Networks[networkName]
	_, network, err := net.ParseCIDR(fmt.Sprintf("%s/%d", dockerNetwork.Gateway, dockerNetwork.IPPrefixLen))
	_, network6, err6 := net.ParseCIDR(fmt.Sprintf("%s/%d", dockerNetwork.IPv6Gateway, dockerNetwork.GlobalIPv6PrefixLen))

	if err != nil {
		return nil, nil, err
	}
	if err6 != nil {
		return nil, nil, err6
	}

	if network == nil || network6 == nil {
		return nil, nil, fmt.Errorf("no addresses found, IPv4Error(\"%s\"), IPv6Error(\"%s\")", err, err6)
	}

	return network, network6, nil
}
