package loadimage

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

func (a *Addon) loadIntoKind(cluster clusters.Cluster) error {
	if len(a.images) == 0 {
		return fmt.Errorf("no images provided")
	}

	deployArgs := []string{
		"load", "docker-image",
		"--name", cluster.Name(),
	}

	deployArgs = append(deployArgs, a.images...)

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kind", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}
	a.loaded = true
	return nil
}
