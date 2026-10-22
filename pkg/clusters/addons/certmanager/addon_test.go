package certmanager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWebhookCAInjected(t *testing.T) {
	// webhookConfigurations returns the two cert-manager webhook configurations, both with the
	// given caBundle (cainjector injects the caBundle into both of them).
	webhookConfigurations := func(caBundle []byte) []runtime.Object {
		clientConfig := admissionregistrationv1.WebhookClientConfig{CABundle: caBundle}
		return []runtime.Object{
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: webhookConfigurationName},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{{Name: "webhook.cert-manager.io", ClientConfig: clientConfig}},
			},
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: webhookConfigurationName},
				Webhooks:   []admissionregistrationv1.ValidatingWebhook{{Name: "webhook.cert-manager.io", ClientConfig: clientConfig}},
			},
		}
	}

	t.Run("injected", func(t *testing.T) {
		injected, err := webhookCAInjected(context.Background(), fake.NewClientset(webhookConfigurations([]byte("ca"))...))
		require.NoError(t, err)
		require.True(t, injected)
	})

	t.Run("not injected yet", func(t *testing.T) {
		injected, err := webhookCAInjected(context.Background(), fake.NewClientset(webhookConfigurations(nil)...))
		require.NoError(t, err)
		require.False(t, injected)
	})

	t.Run("webhook configurations not deployed yet", func(t *testing.T) {
		injected, err := webhookCAInjected(context.Background(), fake.NewClientset())
		require.NoError(t, err)
		require.False(t, injected)
	})
}
