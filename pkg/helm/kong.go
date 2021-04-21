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
// Additional ports will be open on the proxy to support testing TCP and UDP ingress:
//  - 8888 (TCP)
//  - 9999 (UDP)
// If more ports are needed they can be added to the KONG_STREAM_LISTEN env var of the pod
// dynamically, however it would likely be beneficial to just increase the default pool here.
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
		// this function assumes you're bringing your own controller
		"--set", "ingressController.enabled=false",
		"--skip-crds",
		// exposing the admin API and enabling raw HTTP for using it is convenient,
		// but again keep in mind this is meant ONLY for testing scenarios and isn't secure.
		"--set", "proxy.http.nodePort=30080",
		"--set", "admin.enabled=true",
		"--set", "admin.http.enabled=true",
		"--set", "admin.http.nodePort=32080",
		"--set", "admin.tls.enabled=false",
		"--set", "tls.enabled=false",
		// this deployment expects a LoadBalancer Service provisioner (such as MetalLB).
		"--set", "proxy.type=LoadBalancer",
		"--set", "admin.type=LoadBalancer",
		// we set up a few default ports for TCP and UDP proxy stream, it's up to
		// test cases to use these how they see fit AND clean up after themselves.
		"--set", "proxy.stream[0].containerPort=8888",
		"--set", "proxy.stream[0].servicePort=8888",
		"--set", "proxy.stream[1].containerPort=9999",
		"--set", "proxy.stream[1].servicePort=9999",
		"--set", "proxy.stream[1].parameters[0]=udp",
		"--set", "proxy.stream[1].parameters[1]=reuseport",
	)
	install.Stdout = stdout
	install.Stderr = stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("%s: %s: %w", stdout.String(), stderr.String(), err)
	}

	return nil
}
