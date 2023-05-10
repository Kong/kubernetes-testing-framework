package clusters

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestFixupObjKinds(t *testing.T) {
	testcases := []struct {
		name     string
		obj      client.Object
		expected schema.GroupVersionKind
	}{
		{
			name: "gatewayclass",
			obj: &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-gatewayclass",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1beta1",
				Kind:    "GatewayClass",
			},
		},
		{
			name: "gateway",
			obj: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-gateway",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1beta1",
				Kind:    "Gateway",
			},
		},
		{
			name: "httproute",
			obj: &gatewayv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-httproute",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1beta1",
				Kind:    "HTTPRoute",
			},
		},
		{
			name: "tcproute",
			obj: &gatewayv1alpha2.TCPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-tcproute",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1alpha2",
				Kind:    "TCPRoute",
			},
		},
		{
			name: "udproute",
			obj: &gatewayv1alpha2.UDPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-udproute",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1alpha2",
				Kind:    "UDPRoute",
			},
		},
		{
			name: "tlsroute",
			obj: &gatewayv1alpha2.TLSRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-tlsroute",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1alpha2",
				Kind:    "TLSRoute",
			},
		},
		{
			name: "referencegrant",
			obj: &gatewayv1alpha2.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-referencegrant",
				},
			},
			expected: schema.GroupVersionKind{
				Group:   "gateway.networking.k8s.io",
				Version: "v1alpha2",
				Kind:    "ReferenceGrant",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			obj := fixupObjKinds(tc.obj)
			assert.Equal(t, tc.expected.Group, obj.GetObjectKind().GroupVersionKind().Group)
			assert.Equal(t, tc.expected.Kind, obj.GetObjectKind().GroupVersionKind().Kind)
			assert.Equal(t, tc.expected.Version, obj.GetObjectKind().GroupVersionKind().Version)
		})
	}
}

func TestCleanerCanBeUsedConcurrently(*testing.T) {
	cleaner := NewCleaner(nil)
	for i := 0; i < 100; i++ {
		i := i
		go func() {
			cleaner.Add(&corev1.Pod{})
		}()
		go func() {
			cleaner.AddManifest(fmt.Sprintf("manifest-%d.yaml", i))
		}()
		go func() {
			cleaner.AddNamespace(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("ns-%d", i),
				},
			})
		}()
	}
}
