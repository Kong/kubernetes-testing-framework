//+build integration_tests

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apix "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kong/kubernetes-testing-framework/pkg/kind"
)

func TestKongProxyClusterWithMetalLB(t *testing.T) {
	config := kind.ClusterConfigurationWithKongProxy{
		DockerNetwork: kind.DefaultKindDockerNetwork,
		EnableMetalLB: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	cluster, ready, err := config.Deploy(ctx)
	assert.NoError(t, err)
	defer cluster.Cleanup()

	event := <-ready
	assert.NoError(t, event.Err)
	assert.NotEmpty(t, event)

	assert.Eventually(t, func() bool {
		resp, err := http.Get(event.ProxyURL.String())
		if err != nil {
			t.Logf("received error while trying to reach the proxy at %s: %v", event.ProxyURL, err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Logf("expected status code %d, received: %d", http.StatusNotFound, resp.StatusCode)
			return false
		}
		return resp.StatusCode == http.StatusNotFound
	}, time.Minute*3, time.Millisecond*200)

	// the proxy-only configuration should have no pre-installed CRDs present
	c, err := apix.NewForConfig(cluster.Config())
	assert.NoError(t, err)
	crds, err := c.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, crds.Items, 0)

	// the configuration should show that it's configured for dbless
	httpc := http.Client{Timeout: time.Second * 3}
	assert.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("%s/", event.ProxyAdminURL))
		if err != nil {
			t.Logf("WARNING: error while waiting for %s: %v", event.ProxyAdminURL.String(), err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			// buffer the response body
			b := new(bytes.Buffer)
			b.ReadFrom(resp.Body)

			// decode the JSON contents
			decoded := map[string]interface{}{}
			if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
				t.Logf("WARNING: error while unmarshaling JSON from proxy: %v", err)
				return false
			}

			// ensure that the configuration.database field is present
			cfg, ok := decoded["configuration"].(map[string]interface{})
			if !ok {
				return false
			}
			dbmode, ok := cfg["database"].(string)
			if !ok {
				return false
			}

			return dbmode == "off"
		}
		return false
	}, time.Second*30, time.Second*1)
}

func TestKongProxyClusterWithPostgresBackend(t *testing.T) {
	config := kind.ClusterConfigurationWithKongProxy{
		DockerNetwork: kind.DefaultKindDockerNetwork,
		EnableMetalLB: true,
		DBMode:        "postgres",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	cluster, ready, err := config.Deploy(ctx)
	assert.NoError(t, err)
	defer cluster.Cleanup()

	event := <-ready
	assert.NoError(t, event.Err)
	assert.NotEmpty(t, event)

	assert.Eventually(t, func() bool {
		resp, err := http.Get(event.ProxyURL.String())
		if err != nil {
			t.Logf("received error while trying to reach the proxy at %s: %v", event.ProxyURL, err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Logf("expected status code %d, received: %d", http.StatusNotFound, resp.StatusCode)
			return false
		}
		return resp.StatusCode == http.StatusNotFound
	}, time.Minute*3, time.Millisecond*200)

	// the proxy-only configuration should have no pre-installed CRDs present
	c, err := apix.NewForConfig(cluster.Config())
	assert.NoError(t, err)
	crds, err := c.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, crds.Items, 0)

	// the configuration should show that it's configured for postgres
	httpc := http.Client{Timeout: time.Second * 3}
	assert.Eventually(t, func() bool {
		resp, err := httpc.Get(fmt.Sprintf("%s/", event.ProxyAdminURL))
		if err != nil {
			t.Logf("WARNING: error while waiting for %s: %v", event.ProxyAdminURL.String(), err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			// buffer the response body
			b := new(bytes.Buffer)
			b.ReadFrom(resp.Body)

			// decode the JSON contents
			decoded := map[string]interface{}{}
			if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
				t.Logf("WARNING: error while unmarshaling JSON from proxy: %v", err)
				return false
			}

			// ensure that the configuration.database field is present
			cfg, ok := decoded["configuration"].(map[string]interface{})
			if !ok {
				return false
			}
			dbmode, ok := cfg["database"].(string)
			if !ok {
				return false
			}

			return dbmode == "postgres"
		}
		return false
	}, time.Second*30, time.Second*1)
}
