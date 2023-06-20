//go:build integration_tests

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/certmanager"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestCertManagerAddon(t *testing.T) {
	t.Parallel()

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := environments.NewBuilder().WithAddons(certmanager.New()).Build(ctx)
	require.NoError(t, err)

	t.Logf("setting up the environment cleanup for environment %s and cluster %s", env.Name(), env.Cluster().Name())
	defer func() {
		t.Logf("cleaning up environment %s and cluster %s", env.Name(), env.Cluster().Name())
		assert.NoError(t, env.Cleanup(ctx))
	}()

	t.Log("waiting for cluster and addons to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))

	t.Log("deploying a cert-manager certificate to the cluster")
	require.NoError(t, clusters.ApplyManifestByYAML(ctx, env.Cluster(), `---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: certificate-for-integration-tests
  namespace: default
spec:
  secretName: certificate-for-integration-tests
  duration: 2160h # 90d
  renewBefore: 360h # 15d
  subject:
    organizations:
      - kong
  isCA: false
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 4096
  usages:
    - server auth
    - client auth
  dnsNames:
    - konghq.com
    - docs.konghq.com
  issuerRef:
    kind: ClusterIssuer
    name: selfsigned
`))

	t.Log("verifying that the certificate gets provisioned by cert-manager")
	require.NoError(t, clusters.WaitForCondition(ctx, env.Cluster(), corev1.NamespaceDefault, "certificates.cert-manager.io", "certificate-for-integration-tests", "Ready", 30))

	t.Log("ensuring that the certificate secret was created properly")
	require.Eventually(t, func() bool {
		_, err := env.Cluster().Client().CoreV1().Secrets(corev1.NamespaceDefault).Get(ctx, "certificate-for-integration-tests", metav1.GetOptions{})
		return err == nil
	}, time.Minute, time.Second)
}
