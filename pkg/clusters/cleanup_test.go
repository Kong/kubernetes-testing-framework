package clusters

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCleanerCanBeUsedConcurrently(*testing.T) {
	cleaner := NewCleaner(nil, runtime.NewScheme())
	for i := range 100 {
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
