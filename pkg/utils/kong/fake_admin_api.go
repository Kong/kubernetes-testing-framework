package kong

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/kong/go-kong/kong"
)

// -----------------------------------------------------------------------------
// FakeAdminAPIServer - Public Types
// -----------------------------------------------------------------------------

// AdminAPIResponse represents an HTTP response from the Kong Admin API
type AdminAPIResponse struct {
	Status   int
	Body     []byte
	Callback func()
}

// FakeAdminAPIServer implements a basic httptest.Server which can be used as a Kong Admin API for unit tests.
type FakeAdminAPIServer struct {
	// Endpoint is the (fake) HTTP server for the Kong Admin API
	Endpoint *httptest.Server

	// KongClient is a *kong.Client configured to connect with the Endpoint
	KongClient *kong.Client

	// HTTPClient is an *http.Client configured to connect with the Endpoint
	HTTPClient *http.Client

	// mocks is a buffered channel which holds override responses to be produced by the API
	mocks chan AdminAPIResponse
}

// NewFakeAdminAPIServer provides a new *FakeAdminAPIServer which can be used for unit testing functionality
// that requires a *kong.Client or otherwise needs to communicate with the Kong Admin API.
func NewFakeAdminAPIServer() (*FakeAdminAPIServer, error) {
	// start up the fake admin api server
	mocks := make(chan AdminAPIResponse, 1000)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case override := <-mocks:
			// run any callbacks that were configured in the mock (these are optional)
			if override.Callback != nil {
				override.Callback()
			}

			// for most tests you'll want to buffer several mock responses and then run through your requests in the same order
			w.WriteHeader(override.Status)
			fmt.Fprintf(w, string(override.Body))
		default:
			// by default the response behavior is to provide a minimal root configuration for the server
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{
				"version": "0.0.0",
				"configuration": {
					"database": "off"
				}
			}`)
		}
		return
	}))

	// generate an http client for the fake admin api server
	httpc := &http.Client{}
	httpc.Transport = http.DefaultTransport.(*http.Transport)

	// generate a kong client for the fake admin api server
	kongc, err := kong.NewClient(kong.String(endpoint.URL), httpc)
	if err != nil {
		return nil, err
	}

	return &FakeAdminAPIServer{
		Endpoint:   endpoint,
		KongClient: kongc,
		HTTPClient: httpc,
		mocks:      mocks,
	}, nil
}

// -----------------------------------------------------------------------------
// FakeAdminAPIServer - Public Methods
// -----------------------------------------------------------------------------

func (f *FakeAdminAPIServer) MockNextResponse(r AdminAPIResponse) {
	f.mocks <- r
}
