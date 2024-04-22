package clusters

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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
	lock       sync.RWMutex
}

// NewCleaner provides a new initialized *Cleaner object.
func NewCleaner(cluster Cluster) *Cleaner {
	return &Cleaner{cluster: cluster}
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
	dyn, err := dynamic.NewForConfig(c.cluster.Config())
	if err != nil {
		return err
	}

	for _, obj := range c.objects {
		resource := resourceDeleterForObj(dyn, obj)

		if err := resource.Delete(ctx, obj.GetName(), metav1.DeleteOptions{}); err != nil {
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
		namespace := namespace
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

// fixupObjKinds takes a client.Object and checks if it's of one of the gateway
// API types and if so then it adjusts that object's Kind and APIVersion.
// This possibly might also need other types to be included but those are enough
// for our needs for now especially since that will help cleaning up non-namespaced
// GatewayClasses which are not cleaned up on namespace removal also done in
// Cleanup().
//
// The reason we need this is that when decoding to go structs APIVersion and Kind
// are dropper because the type info is inherent in the object.
// Decoding to unstructured objects (like the dynamic client does) preserves that
// information.
// There should be a better way of doing this.
//
// Possibly related:
// - https://github.com/kubernetes/kubernetes/issues/3030
// - https://github.com/kubernetes/kubernetes/issues/80609
func fixupObjKinds(obj client.Object) client.Object {
	// If Kind and APIVersion are set then we're good.
	if obj.GetObjectKind().GroupVersionKind().Kind != "" && obj.GetResourceVersion() != "" {
		return obj
	}

	// Otherwise try to fix that up by performing type assertions and filling
	// those 2 fields accordingly.
	switch o := obj.(type) {
	case *gatewayv1.GatewayClass:
		o.Kind = "GatewayClass"
		o.APIVersion = gatewayv1.GroupVersion.String()
		return o
	case *gatewayv1.Gateway:
		o.Kind = "Gateway"
		o.APIVersion = gatewayv1.GroupVersion.String()
		return o
	case *gatewayv1.HTTPRoute:
		o.Kind = "HTTPRoute"
		o.APIVersion = gatewayv1.GroupVersion.String()
		return o

	case *gatewayv1alpha2.TCPRoute:
		o.Kind = "TCPRoute"
		o.APIVersion = gatewayv1alpha2.GroupVersion.String()
		return o
	case *gatewayv1alpha2.UDPRoute:
		o.Kind = "UDPRoute"
		o.APIVersion = gatewayv1alpha2.GroupVersion.String()
		return o
	case *gatewayv1alpha2.TLSRoute:
		o.Kind = "TLSRoute"
		o.APIVersion = gatewayv1alpha2.GroupVersion.String()
		return o
	case *gatewayv1beta1.ReferenceGrant:
		o.Kind = "ReferenceGrant"
		o.APIVersion = gatewayv1beta1.GroupVersion.String()
		return o

	default:
		return obj
	}
}

type deleter interface {
	Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error
}

func resourceDeleterForObj(dyn *dynamic.DynamicClient, obj client.Object) deleter {
	obj = fixupObjKinds(obj)

	var (
		namespace = obj.GetNamespace()
		kind      = obj.GetObjectKind()
		gvk       = kind.GroupVersionKind()
	)

	var gvr schema.GroupVersionResource
	switch gvk.Kind {
	// GatewayClass is a special case because gatewayclass + "s" is not a plural
	// of gatewayclass.
	case "GatewayClass":
		gvr = schema.GroupVersionResource{
			Group:    gvk.Group,
			Version:  gvk.Version,
			Resource: "gatewayclasses",
		}
	default:
		res := strings.ToLower(gvk.Kind) + "s"
		gvr = gvk.GroupVersion().WithResource(res)
	}

	if namespace == "" {
		return dyn.Resource(gvr)
	}
	return dyn.Resource(gvr).Namespace(namespace)
}

// DumpDiagnostics dumps diagnostics from the underlying cluster.
//
// Deprecated: Users should use Cluster.DumpDiagnostics().
func (c *Cleaner) DumpDiagnostics(ctx context.Context, meta string) (string, error) {
	return c.cluster.DumpDiagnostics(ctx, meta)
}
