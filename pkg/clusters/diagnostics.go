package clusters

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DiagnosticOutDirectoryPrefix is the tmpdir prefix used for diagnostic dumps.
const DiagnosticOutDirectoryPrefix = "ktf-diag-"

// DumpAllDescribeAll gathers diagnostic information from the cluster.
// Specifically it runs "kubectl get all" and "kubectl describe all" for
// all resources and stores the output into two respective yaml files
// (kubectl_get_all.yaml and kubectl_describe_all.yaml).
func DumpAllDescribeAll(ctx context.Context, c Cluster, outDir string) error {
	// Obtain a kubeconfig
	kubeconfig, err := TempKubeconfig(c)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	// kubectl api-resources --verbs=list --namespaced -o name  | xargs -n 1 kubectl get --show-kind --ignore-not-found -A -oyaml
	// kubectl api-resources --verbs=list --namespaced -o name  | xargs -n 1 kubectl get --show-kind --ignore-not-found -A -oyaml
	// aka "kubectl get all" and "kubectl describe all", but also gets CRs and cluster-scoped resouces
	getAllOut, err := os.OpenFile(filepath.Join(outDir, "kubectl_get_all.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gomnd
	if err != nil {
		return err
	}
	defer getAllOut.Close()
	describeAllOut, err := os.OpenFile(filepath.Join(outDir, "kubectl_describe_all.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gomnd
	if err != nil {
		return err
	}
	defer describeAllOut.Close()

	var namespacedList bytes.Buffer
	var clusterList bytes.Buffer
	namespacedResources := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "api-resources", "--verbs=list", "--namespaced", "-o", "name") //nolint:gosec
	namespacedResources.Stdout = &namespacedList
	clusterResources := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "api-resources", "--verbs=list", "--namespaced=false", "-o", "name") //nolint:gosec
	clusterResources.Stdout = &clusterList
	if err := namespacedResources.Run(); err != nil {
		return err
	}
	if err := clusterResources.Run(); err != nil {
		return err
	}
	combinedList := strings.Split(namespacedList.String()+clusterList.String(), "\n")

	// run kubectl get all and kubectl describe all for each resource.
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
			return fmt.Errorf("could not get resources for cmd '%s': err %s, stderr: %s", resourceGet.String(), err, getErr.String())
		}
		if err := resourceDescribe.Run(); err != nil {
			return fmt.Errorf("could not get resources for cmd '%s': err %s, stderr: %s", resourceDescribe.String(), err, descErr.String())
		}
	}

	return nil
}

// DumpDiagnostics gathers a wide range of generic, diagnostic information from the test cluster,
// to provide a snapshot of it at a given time for offline debugging.
// It uses the provided context and writes the meta string to meta.txt to identify the result set.
// It returns the name of the directory that contains all the produced diagnostics data.
func DumpDiagnostics(ctx context.Context, c Cluster, meta string, outDir string) error {
	// for each Addon, run the addon diagnostic function
	failedAddons := make(map[string]error)
	for _, addon := range c.ListAddons() {
		diagnostics, err := addon.DumpDiagnostics(ctx, c)
		if err != nil {
			failedAddons[string(addon.Name())] = err
			continue
		}
		if len(diagnostics) > 0 {
			addonOut := filepath.Join(outDir, "addons", string(addon.Name()))
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
		failedAddonOut, err := os.Create(filepath.Join(outDir, "addon_failures.txt"))
		if err != nil {
			return err
		}
		defer failedAddonOut.Close()
		for failed, reason := range failedAddons {
			_, err = failedAddonOut.WriteString(fmt.Sprintf("%s: %v\n", failed, reason))
			if err != nil {
				return err
			}
		}
	}

	// write the diagnostic metadata
	metaOut, err := os.Create(filepath.Join(outDir, "meta.txt"))
	if err != nil {
		return err
	}
	defer metaOut.Close()
	_, err = metaOut.WriteString(meta)
	if err != nil {
		return err
	}

	err = DumpAllDescribeAll(ctx, c, outDir)
	// write errors if we failed to dump results of `kubectl get all` or `kubectl describe all`.
	// in cases where kubernetes cluster may not be correctly created.
	if err != nil {
		kubectlErrorOut, openErr := os.OpenFile(filepath.Join(outDir, "kubectl_dump_error.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gomnd
		if openErr != nil {
			return openErr
		}
		defer kubectlErrorOut.Close()
		_, writeErr := kubectlErrorOut.WriteString(err.Error())
		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}
