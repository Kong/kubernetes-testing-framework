package clusters

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// -----------------------------------------------------------------------------
// Cleaner - Public
// -----------------------------------------------------------------------------

// Cleaner holds namespaces and objects for later cleanup. This is generally
// used during integration tests to clean up test resources.
type Cleaner struct {
	cluster    Cluster
	scheme     *runtime.Scheme
	objects    []client.Object
	manifests  []string
	namespaces []*corev1.Namespace
	lock       sync.RWMutex
}

// NewCleaner provides a new initialized *Cleaner object.
func NewCleaner(cluster Cluster, scheme *runtime.Scheme) *Cleaner {
	return &Cleaner{
		cluster: cluster,
		scheme:  scheme,
	}
}

// -----------------------------------------------------------------------------
// Cleaner - Public
// -----------------------------------------------------------------------------

func (c *Cleaner) Add(obj client.Object) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.objects = append([]client.Object{obj}, c.objects...)
}

func (c *Cleaner) AddManifest(manifest string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.manifests = append(c.manifests, manifest)
}

func (c *Cleaner) AddNamespace(namespace *corev1.Namespace) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.namespaces = append(c.namespaces, namespace)
}

func (c *Cleaner) Cleanup(ctx context.Context) error {
	c.lock.RLock()
	defer c.lock.RUnlock()

	cl, err := client.New(c.cluster.Config(), client.Options{
		Scheme: c.scheme,
	})
	if err != nil {
		return err
	}

	for _, obj := range c.objects {
		if err := cl.Delete(ctx, obj); err != nil {
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
	g.SetLimit(8) //nolint:mnd

	for _, namespace := range c.namespaces {
		g.Go(func() error {
			namespaceClient := c.cluster.Client().CoreV1().Namespaces()

			if err := namespaceClient.Delete(ctx, namespace.Name, metav1.DeleteOptions{}); err != nil {
				if errors.IsNotFound(err) {
					// If the namespace cannot be found then we're good to go.
					return nil
				}
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
// Deprecated: Users should use Cluster.DumpDiagnostics().
func (c *Cleaner) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	return c.cluster.DumpDiagnostics(ctx, meta)
}
