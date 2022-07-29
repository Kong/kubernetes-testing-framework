package clusters

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// -----------------------------------------------------------------------------
// Cleaner - Public
// -----------------------------------------------------------------------------

// Cleaner holds namespaces and objects for later cleanup. This is generally
// used during integration tests to clean up test resources.
type Cleaner struct {
	cluster    Cluster
	objects    []client.Object
	namespaces []*corev1.Namespace
}

// NewCleaner provides a new initialized *Cleaner object.
func NewCleaner(cluster Cluster) *Cleaner {
	return &Cleaner{cluster: cluster}
}

// -----------------------------------------------------------------------------
// Cleaner - Public
// -----------------------------------------------------------------------------

func (c *Cleaner) Add(obj client.Object) {
	c.objects = append([]client.Object{obj}, c.objects...)
}

func (c *Cleaner) AddNamespace(namespace *corev1.Namespace) {
	c.namespaces = append(c.namespaces, namespace)
}

// DumpDiagnostics gathers a wide range of diagnostic information from the test cluster, to provide a snapshot of it
// at a given time for offline debugging. It uses the provided context and writes the meta string to meta.txt to
// identify the result set.
func (c *Cleaner) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	// Obtain a kubeconfig
	kubeconfig, err := TempKubeconfig(c.cluster)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfig.Name())

	// create a tempdir
	output, err := os.MkdirTemp(os.TempDir(), "ktf-diag-")
	if err != nil {
		return "", err
	}

	// kubectl get all --all-namespaces -oyaml
	getAllOut, err := os.Create(filepath.Join(output, "kubectl_get_all.yaml"))
	if err != nil {
		return output, err
	}
	defer getAllOut.Close()
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "get", "all", "--all-namespaces", "-o", "yaml") //nolint:gosec
	cmd.Stdout = getAllOut
	if err := cmd.Run(); err != nil {
		return output, err
	}

	// kubectl describe all --all-namespaces
	describeAllOut, err := os.Create(filepath.Join(output, "kubectl_describe_all.txt"))
	if err != nil {
		return output, err
	}
	cmd = exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig.Name(), "get", "all", "--all-namespaces", "-o", "yaml") //nolint:gosec
	cmd.Stdout = describeAllOut
	if err := cmd.Run(); err != nil {
		return output, err
	}
	defer describeAllOut.Close()

	// for each Pod, run kubectl logs
	pods, err := c.cluster.Client().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return output, err
	}
	logsDir := filepath.Join(output, "pod_logs")
	err = os.Mkdir(logsDir, 0750) //nolint:gomnd
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
	for _, addon := range c.cluster.ListAddons() {
		diagnostics, err := addon.DumpDiagnostics(ctx, c.cluster)
		if err != nil {
			failedAddons[string(addon.Name())] = err
			continue
		}
		if len(diagnostics) > 0 {
			addonOut := filepath.Join(output, "addons", string(addon.Name()))
			err = os.MkdirAll(addonOut, 0750) //nolint:gomnd
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

func (c *Cleaner) Cleanup(ctx context.Context) error {
	dyn, err := dynamic.NewForConfig(c.cluster.Config())
	if err != nil {
		return err
	}

	for _, obj := range c.objects {
		namespace := obj.GetNamespace()
		name := obj.GetName()
		res := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind) + "s"
		gvr := obj.GetObjectKind().GroupVersionKind().GroupVersion().WithResource(res)
		resource := dyn.Resource(gvr).Namespace(namespace)
		if err := resource.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	for _, namespace := range c.namespaces {
		if err := c.cluster.Client().CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}
