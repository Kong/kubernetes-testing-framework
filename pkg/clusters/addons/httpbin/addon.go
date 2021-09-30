package httpbin

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/google/uuid"
	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
)

// -----------------------------------------------------------------------------
// HttpBin Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the HttpBin cluster.Addon
	AddonName clusters.AddonName = "httpbin"

	// DefaultNamespace is the namespace that the Addon components will be deployed
	DefaultNamespace = "httpbin"

	// Image is the container image that will be used by default.
	Image = "kennethreitz/httpbin"

	// DefaultPort is the port that will be used for the HttpBin endpoint
	// on pods and services unless otherwise specified.
	DefaultPort = 80
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	name               string
	namespace          string
	generateNamespace  bool
	ingressAnnotations map[string]string
	path               string
}

// New produces a new clusters.Addon for Kong but uses a very opionated set of
// default configurations (see the defaults() function for more details).
// If you need to customize your Kong deployment, use the kong.Builder instead.
func New() *Addon {
	return NewBuilder().Build()
}

// Namespace indicates the namespace where the HttpBin addon components are to be
// deployed and managed.
func (a *Addon) Namespace() string {
	return a.namespace
}

// -----------------------------------------------------------------------------
// HttpBin Addon - Public Methods
// -----------------------------------------------------------------------------

// Path provides the URL path which the addon can be reached via Ingress.
func (a *Addon) Path() string {
	return a.path
}

// -----------------------------------------------------------------------------
// HttpBin Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// generate a namespace name if the caller optioned for that
	if a.generateNamespace {
		a.namespace = uuid.New().String()
	}
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.namespace}}

	// ensure the namespace for this addon is available
	_, err := cluster.Client().CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// determine the kubernetes cluster version so we know if we need to use a legacy ingress API
	kubernetesVersion, err := cluster.Version()
	if err != nil {
		return err
	}

	// generate a container, deployment, service and ingress resource for the HttpBin addon
	a.path = fmt.Sprintf("/%s", a.name)
	container := generators.NewContainer(a.name, Image, DefaultPort)
	deployment, service, ingress := generators.NewIngressForContainerWithDeploymentAndService(
		kubernetesVersion,
		container,
		corev1.ServiceTypeClusterIP,
		a.ingressAnnotations,
		a.path,
	)

	// deploy the httpbin deployment
	_, err = cluster.Client().AppsV1().Deployments(a.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// expose httpbin inside the cluster via service
	_, err = cluster.Client().CoreV1().Services(a.namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// expose httpbin outside the cluster via ingress
	if err := clusters.DeployIngress(ctx, cluster, a.namespace, ingress); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed before addon could be deleted: %w", ctx.Err())
		default:
			if err := cluster.Client().CoreV1().Namespaces().Delete(ctx, a.namespace, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
	}
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	return utils.IsNamespaceAvailable(ctx, cluster, a.namespace)
}
