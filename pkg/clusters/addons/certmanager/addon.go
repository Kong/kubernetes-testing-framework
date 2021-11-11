package certmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/blang/semver/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/github"
)

// -----------------------------------------------------------------------------
// CertManager Addon - Builder
// -----------------------------------------------------------------------------

type Builder struct {
	version *semver.Version
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithVersion(version semver.Version) *Builder {
	b.version = &version
	return b
}

func (b *Builder) Build() *Addon {
	return &Addon{
		version: b.version,
	}
}

// -----------------------------------------------------------------------------
// CertManager Addon
// -----------------------------------------------------------------------------

const (
	// AddonName indicates the unique name of this addon.
	AddonName clusters.AddonName = "cert-manager"

	// DefaultNamespace indicates the default namespace this addon will be deployed to.
	DefaultNamespace = "cert-manager"
)

type Addon struct {
	version *semver.Version
}

func New() clusters.Addon {
	return &Addon{}
}

// -----------------------------------------------------------------------------
// CertManager Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	var err error
	if a.version == nil {
		a.version, err = github.FindLatestReleaseForRepo("jetstack", "cert-manager")
		if err != nil {
			return err
		}
	}

	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"apply", "-f", fmt.Sprintf(manifestFormatter, a.version),
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	// we need to wait for deployment readiness before we try to deploy a
	// default issuer for the cluster.
	deploymentsReady := false
	for !deploymentsReady {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed before deployment could complete: %w", ctx.Err())
		default:
			_, deploymentsReady, err = a.Ready(ctx, cluster)
			if err != nil {
				return err
			}
		}
	}

	return a.deployDefaultIssuer(ctx, cluster)
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	kubeconfig, err := clusters.TempKubeconfig(cluster)
	if err != nil {
		return err
	}
	defer os.Remove(kubeconfig.Name())

	deployArgs := []string{
		"--kubeconfig", kubeconfig.Name(),
		"delete", "-f", fmt.Sprintf(manifestFormatter, a.version),
	}

	stderr := new(bytes.Buffer)
	cmd := exec.Command("kubectl", deployArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return a.cleanupDefaultIssuer(ctx, cluster)
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) ([]runtime.Object, bool, error) {
	for _, deploymentName := range []string{"cert-manager", "cert-manager-cainjector", "cert-manager-webhook"} {
		deployment, err := cluster.Client().AppsV1().Deployments(DefaultNamespace).
			Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return nil, false, err
		}

		if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			return []runtime.Object{deployment}, false, nil
		}
	}

	return nil, true, nil
}

// -----------------------------------------------------------------------------
// CertManager Addon - Private
// -----------------------------------------------------------------------------

const (
	manifestFormatter        = "https://github.com/jetstack/cert-manager/releases/download/v%s/cert-manager.yaml"
	defaultIssuerWaitSeconds = 60
	defaultIssuer            = `---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned
spec:
  selfSigned: {}
`
)

func (a *Addon) deployDefaultIssuer(ctx context.Context, cluster clusters.Cluster) error {
	if err := clusters.ApplyYAML(ctx, cluster, defaultIssuer); err != nil {
		return err
	}
	return clusters.WaitForCondition(ctx, cluster, DefaultNamespace, "clusterissuers.cert-manager.io", "selfsigned", "Ready", defaultIssuerWaitSeconds)
}

func (a *Addon) cleanupDefaultIssuer(ctx context.Context, cluster clusters.Cluster) error {
	return clusters.DeleteYAML(ctx, cluster, defaultIssuer)
}
