package docker

import "fmt"

// -----------------------------------------------------------------------------
// Public Vars - Kind
// -----------------------------------------------------------------------------

var (
	// KindContainerSuffix provides the string suffix that Kind names all cluster containers with.
	KindContainerSuffix = "-control-plane"

	// DefaultKindNetwork is the name of the default Docker network used for Kind clusters
	DefaultKindNetwork = "kind"
)

// -----------------------------------------------------------------------------
// Public Functions - Kind Helpers
// -----------------------------------------------------------------------------

// GetKindContainerID produces the docker container ID for the given kind cluster by name.
func GetKindContainerID(clusterName string) string {
	return fmt.Sprintf("%s%s", clusterName, KindContainerSuffix)
}

// GetContainerIP retrieves the IPv4 address of a Kind container given the cluster name.
func GetKindContainerIP(clusterName string) (string, error) {
	containerID := GetKindContainerID(clusterName)

	containerJSON, err := InspectDockerContainer(containerID)
	if err != nil {
		return "", err
	}

	kindNetwork, ok := containerJSON.NetworkSettings.Networks[DefaultKindNetwork]
	if !ok {
		return "", fmt.Errorf("missing docker container network %s", DefaultKindNetwork)
	}

	return kindNetwork.IPAddress, nil
}
