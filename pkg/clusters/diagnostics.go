package clusters

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DumpDiagnostics gathers a wide range of generic, diagnostic information from the test cluster,
// to provide a snapshot of it at a given time for offline debugging.
// It uses the provided context and writes the meta string to meta.txt to identify the result set.
// It returns the name of the directory that contains all the produced diagnostics data.
func DumpDiagnostics(ctx context.Context, c Cluster, meta string) (string, error) {
	// Obtain a kubeconfig
	kubeconfig, err := TempKubeconfig(c)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfig.Name())

	// create a tempdir
	output, err := os.MkdirTemp(os.TempDir(), "ktf-diag-")
	if err != nil {
		return "", err
	}

	// kubectl api-resources --verbs=list --namespaced -o name  | xargs -n 1 kubectl get --show-kind --ignore-not-found -A -oyaml
	// kubectl api-resources --verbs=list --namespaced -o name  | xargs -n 1 kubectl get --show-kind --ignore-not-found -A -oyaml
	// aka "kubectl get all" and "kubectl describe all", but also gets CRs and cluster-scoped resouces
	getAllOut, err := os.OpenFile(filepath.Join(output, "kubectl_get_all.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gomnd
	if err != nil {
		return output, err
	}
	defer getAllOut.Close()
	describeAllOut, err := os.OpenFile(filepath.Join(output, "kubectl_describe_all.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gomnd
	if err != nil {
		return output, err
	}
	defer describeAllOut.Close()

	var namespacedList bytes.Buffer
	var clusterList bytes.Buffer
	namespacedResources := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "api-resources", "--verbs=list", "--namespaced", "-o", "name") //nolint:gosec
	namespacedResources.Stdout = &namespacedList
	clusterResources := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "api-resources", "--verbs=list", "--namespaced=false", "-o", "name") //nolint:gosec
	clusterResources.Stdout = &clusterList
	if err := namespacedResources.Run(); err != nil {
		return output, err
	}
	if err := clusterResources.Run(); err != nil {
		return output, err
	}
	combinedList := strings.Split(namespacedList.String()+clusterList.String(), "\n")

	for _, resource := range combinedList {
		if resource == "" {
			// unwanted artifact of the split
			continue
		}
		var getErr bytes.Buffer
		var descErr bytes.Buffer
		resourceGet := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "get", "--show-kind", "--ignore-not-found", "-A", "-oyaml", resource) //nolint:gosec
		resourceGet.Stdout = getAllOut
		resourceGet.Stderr = &getErr
		resourceDescribe := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "describe", "--all-namespaces", resource) //nolint:gosec
		resourceDescribe.Stdout = describeAllOut
		resourceDescribe.Stderr = &descErr
		if err := resourceGet.Run(); err != nil {
			return output, fmt.Errorf("could not get resources for cmd '%s': err %s, stderr: %s", resourceGet.String(), err, getErr.String())
		}
		if err := resourceDescribe.Run(); err != nil {
			return output, fmt.Errorf("could not get resources for cmd '%s': err %s, stderr: %s", resourceDescribe.String(), err, descErr.String())
		}
	}

	// for each Pod, run kubectl logs
	pods, err := c.Client().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return output, err
	}
	logsDir := filepath.Join(output, "pod_logs")
	err = os.Mkdir(logsDir, 0o750) //nolint:gomnd
	if err != nil {
		return output, err
	}
	failedPods := make(map[string]error)
	for _, pod := range pods.Items {
		podLogOut, err := os.Create(filepath.Join(logsDir, fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)))
		if err != nil {
			failedPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = err
			continue
		}
		cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "logs", "--all-containers", "-n", pod.Namespace, pod.Name) //nolint:gosec
		cmd.Stdout = podLogOut
		if err := cmd.Run(); err != nil {
			failedPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = err
			continue
		}
		defer podLogOut.Close()
	}
	if len(failedPods) > 0 {
		failedPodOut, err := os.Create(filepath.Join(output, "pod_logs_failures.txt"))
		if err != nil {
			return output, err
		}
		defer failedPodOut.Close()
		for failed, reason := range failedPods {
			_, err = failedPodOut.WriteString(fmt.Sprintf("%s: %v\n", failed, reason))
			if err != nil {
				return output, err
			}
		}
	}

	// for each Addon, run the addon diagnostic function
	failedAddons := make(map[string]error)
	for _, addon := range c.ListAddons() {
		diagnostics, err := addon.DumpDiagnostics(ctx, c)
		if err != nil {
			failedAddons[string(addon.Name())] = err
			continue
		}
		if len(diagnostics) > 0 {
			addonOut := filepath.Join(output, "addons", string(addon.Name()))
			err = os.MkdirAll(addonOut, 0o750) //nolint:gomnd
			if err != nil {
				failedAddons[string(addon.Name())] = err
				continue
			}
			for filename, content := range diagnostics {
				diagOut, err := os.Create(filepath.Join(addonOut, filename))
				if err != nil {
					failedAddons[string(addon.Name())] = err
					continue
				}
				defer diagOut.Close()
				_, err = diagOut.Write(content)
				if err != nil {
					failedAddons[string(addon.Name())] = err
					continue
				}
			}
		}
	}
	if len(failedAddons) > 0 {
		failedAddonOut, err := os.Create(filepath.Join(output, "addon_failures.txt"))
		if err != nil {
			return output, err
		}
		defer failedAddonOut.Close()
		for failed, reason := range failedAddons {
			_, err = failedAddonOut.WriteString(fmt.Sprintf("%s: %v\n", failed, reason))
			if err != nil {
				return output, err
			}
		}
	}

	// write the diagnostic metadata
	metaOut, err := os.Create(filepath.Join(output, "meta.txt"))
	if err != nil {
		return output, err
	}
	defer metaOut.Close()
	_, err = metaOut.WriteString(meta)
	if err != nil {
		return output, err
	}

	return output, nil
}
