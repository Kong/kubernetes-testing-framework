package helm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

const kongChartsRepo = "https://charts.konghq.com"

// DeployKongProxyOnly deploys the Kong Proxy (without KIC) to the Kind cluster provided by name.
func DeployKongProxyOnly(clusterName string) error {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)

	repoAdd := exec.Command("helm", "repo", "add", "kong", kongChartsRepo)
	repoAdd.Stderr = stderr
	if err := repoAdd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	repoUpdate := exec.Command("helm", "repo", "update")
	repoUpdate.Stderr = stderr
	if err := repoUpdate.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	tmpFile, err := ioutil.TempFile(os.TempDir(), "kubeconfig-")
	if err != nil {
		return fmt.Errorf("could not create tempfile: %w", err)
	}
	defer tmpFile.Close()

	configure := exec.Command("kind", "get", "kubeconfig", "--name", clusterName)
	configure.Stdout = tmpFile
	configure.Stderr = stderr
	if err := configure.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	install := exec.Command("helm", "install", "ingress-controller", "kong/kong",
		"--kubeconfig", tmpFile.Name(),
		"--create-namespace", "--namespace", "kong-system",
		"--set", "ingressController.enabled=false",
		"--set", "proxy.type=NodePort",
		"--set", "proxy.http.nodePort=30080",
		"--set", "admin.enabled=true",
		"--set", "admin.http.enabled=true",
		"--set", "admin.http.nodePort=32080",
		"--set", "admin.tls.enabled=false",
		"--set", "tls.enabled=false",
	)
	install.Stdout = stdout
	install.Stderr = stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("%s: %s: %w", stdout.String(), stderr.String(), err)
	}

	return nil
}
