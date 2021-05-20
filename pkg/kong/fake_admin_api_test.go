package kong_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kong/go-kong/kong"
	kongt "github.com/kong/kubernetes-testing-framework/pkg/kong"
)

func TestFakeAdminAPI(t *testing.T) {
	t.Log("starting the fake admin api server up")
	admin, err := kongt.NewFakeAdminAPIServer()
	assert.NoError(t, err)
	assert.NotNil(t, admin)

	t.Log("verifying basic connectivity to the admin api by pulling the root config")
	root, err := admin.KongClient.Root(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "0.0.0", kong.VersionFromInfo(root))

	t.Log("configuring several mocked responses responses from the api")
	callbackExecuted := false
	mocks := []kongt.AdminAPIResponse{
		{
			Status: http.StatusOK,
			Body:   []byte(`{}`),
		},
		{
			Status:   http.StatusCreated,
			Body:     []byte{},
			Callback: func() { callbackExecuted = true },
		},
		{
			Status: http.StatusNotFound,
			Body:   []byte(`error`),
		},
	}

	t.Log("verifying that the mocked responses loaded and worked properly")
	for _, mock := range mocks {
		admin.MockNextResponse(mock)
	}
	for _, mock := range mocks {
		// make sure we got a valid response
		resp, err := admin.HTTPClient.Get(admin.Endpoint.URL)
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, mock.Status, resp.StatusCode)

		// check the consistenty of the response body
		buf := new(bytes.Buffer)
		c, err := buf.ReadFrom(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, len(mock.Body), int(c))
		assert.Equal(t, mock.Body, buf.Bytes())
	}
	assert.True(t, callbackExecuted)

	t.Log("verifying that the mock response buffer was drained")
	root, err = admin.KongClient.Root(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "0.0.0", kong.VersionFromInfo(root))
}
