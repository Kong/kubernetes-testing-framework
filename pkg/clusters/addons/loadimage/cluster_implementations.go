package loadimage

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

func (a *Addon) loadIntoKind(cluster clusters.Cluster) error {
	deployArgs := []string{
		"load", "docker-image",
		a.image,
		"--name", cluster.Name(),
	}

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
