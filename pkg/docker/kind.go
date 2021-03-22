package docker

import "fmt"

// -----------------------------------------------------------------------------
// Public Vars - Kind
// -----------------------------------------------------------------------------

var (
	// KindContainerSuffix provides the string suffix that Kind names all cluster containers with.
	KindContainerSuffix = "-control-plane"
)

// -----------------------------------------------------------------------------
// Public Functions - Kind Helpers
// -----------------------------------------------------------------------------

// GetKindContainerID produces the docker container ID for the given kind cluster by name.
func GetKindContainerID(clusterName string) string {
	return fmt.Sprintf("%s%s", clusterName, KindContainerSuffix)
}
