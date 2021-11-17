package certmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/blang/semver/v4"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
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

func (a *Addon) Dependencies(_ context.Context, _ clusters.Cluster) []clusters.AddonName {
	return nil
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

	if err := a.deployWebhookWaitJob(ctx, cluster); err != nil {
		return err
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

	// delete any webhook wait job that may remain
	if err := cluster.Client().BatchV1().Jobs(DefaultNamespace).Delete(ctx, webhookWaitJobName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) { // tolerate the job having already been deleted
			return err
		}
	}

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
	// wait for all the deployments, daemonsets in the namespace
	waitForObjects, ready, err := utils.IsNamespaceAvailable(ctx, cluster, DefaultNamespace)
	if !ready || err != nil {
		return waitForObjects, ready, err
	}

	// in addition to deployments we wait for our webhook wait job to complete
	// to avoid any timing issues with the webhook webserver and to ensure it
	// is responding to HTTP requests.
	job, err := cluster.Client().BatchV1().Jobs(DefaultNamespace).Get(ctx, webhookWaitJobName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []runtime.Object{job}, false, nil // wait for the job to exist
		}
		return []runtime.Object{job}, false, err
	}
	if job.Status.Succeeded < 1 {
		return []runtime.Object{job}, false, nil // not quite ready yet
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

const webhookWaitJobName = "cert-manager-webhook-wait"

func (a *Addon) deployWebhookWaitJob(ctx context.Context, cluster clusters.Cluster) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookWaitJobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "curl",
						Image:   "curlimages/curl",
						Command: []string{"curl", "-k", fmt.Sprintf("https://cert-manager-webhook.%s.svc/mutate", DefaultNamespace)},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}

	_, err := cluster.Client().BatchV1().Jobs(DefaultNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create job to wait for cert-manager webhook: %w", err)
	}

	return nil
}
