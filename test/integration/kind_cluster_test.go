//+build integration_tests

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kong/kubernetes-testing-framework/pkg/kind"
)

func TestKongProxyClusterWithMetalLB(t *testing.T) {
	config := kind.ClusterConfigurationWithKongProxy{
		DockerNetwork: kind.DefaultKindDockerNetwork,
		EnableMetalLB: true,
	}

	cluster, ready, err := config.Deploy(context.Background(), time.Now().Add(time.Minute*5))
	assert.NoError(t, err)
	defer cluster.Cleanup()

	event := <-ready
	assert.NoError(t, event.Err)
	assert.NotEmpty(t, event)

	assert.Eventually(t, func() bool {
		resp, err := http.Get(event.URL.String())
		if err != nil {
			t.Logf("received error while trying to reach the proxy at %s: %v", event.URL, err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Logf("expected status code %d, received: %d", http.StatusNotFound, resp.StatusCode)
			return false
		}
		return resp.StatusCode == http.StatusNotFound
	}, time.Minute*3, time.Millisecond*200)
}
