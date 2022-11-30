package clusters

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
	manifests  []string
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

func (c *Cleaner) AddManifest(manifest string) {
	c.manifests = append(c.manifests, manifest)
}

func (c *Cleaner) AddNamespace(namespace *corev1.Namespace) {
	c.namespaces = append(c.namespaces, namespace)
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

	for _, manifest := range c.manifests {
		err := DeleteManifestByYAML(ctx, c.cluster, manifest)
		if err != nil {
			return err
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	// Limit the concurrency level to not overwhelm the API server.
	g.SetLimit(8) //nolint:gomnd

	for _, namespace := range c.namespaces {
		namespace := namespace
		g.Go(func() error {
			namespaceClient := c.cluster.Client().CoreV1().Namespaces()

			if err := namespaceClient.Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
				return err
			}

			w, err := namespaceClient.Watch(ctx, metav1.ListOptions{
				LabelSelector: "kubernetes.io/metadata.name=" + namespace.Name,
			})
			if err != nil {
				return err
			}

			defer w.Stop()
			for {
				select {
				case event := <-w.ResultChan():
					if event.Type == watch.Deleted {
						return nil
					}

				case <-ctx.Done():
					return fmt.Errorf(
						"failed to delete namespace %q because the context is done", namespace.Name,
					)
				}
			}
		})
	}

	return g.Wait()
}

// DumpDiagnostics dumps diagnostics from the underlying cluster.
//
// Deprecated. Users should use Cluster.DumpDiagnostics().
func (c *Cleaner) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	return c.cluster.DumpDiagnostics(ctx, meta)
}
