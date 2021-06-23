package kind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// -----------------------------------------------------------------------------
// Kong Cluster - Proxy - Events
// -----------------------------------------------------------------------------

// ProxyReadinessEvent indicates the result of exposing the Kong proxy service in a Cluster
type ProxyReadinessEvent struct {
	// ProxyAdminURL indicates the URL at which the Kong Proxy Admin API can be reached.
	ProxyAdminURL *url.URL

	// ProxyURL indicates the URL at which the Kong proxy can be reached over HTTP.
	ProxyURL *url.URL

	// ProxyHTTPSURL indicates the URL at which the Kong proxy can be reached over HTTPS.
	ProxyHTTPSURL *url.URL

	// ProxyUDPUrl indicates the URL at which UDP traffic the Kong proxy goes.
	// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
	//       See: https://github.com/Kong/charts/issues/329
	ProxyUDPUrl *url.URL

	// ProxyIP is the proxy's IP
	ProxyIP *net.IP

	// Err provides any errors that have occurred that have made it impossible for the Proxy
	// to become ready, receivers should consider any errors received this way as a critical
	// failure that can not be automatically recovered from (e.g. the tests have failed).
	Err error
}

// -----------------------------------------------------------------------------
// Kong Cluster - Proxy - Informers
// -----------------------------------------------------------------------------

// ProxyReadinessInformer provides a channel indicates when the proxy server is fully functional and accessible
// by providing the *url.URLs by which to access it. This channel will publish a single readiness event OR an error and close.
//
// The caller of this function needs to be responsible for ensuring that the proxy deployment and services referred to by the
// proxyDeploymentName variable are present on the cluster, as the informer will fail if any resource is 404 Not Found.
//
// This function relies on the provided context for any timeout that the caller wishes to enforce.
func (c *kongProxyCluster) ProxyReadinessInformer(ctx context.Context, ready chan ProxyReadinessEvent) {
	defer close(ready)
	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == nil {
				err = fmt.Errorf("context completed before proxy was ready")
			}
			ready <- ProxyReadinessEvent{Err: err}
			return
		case <-time.After(ProxyReadinessWaitTick):
			// -------------------------------------------------------------
			// Deployment Check
			// -------------------------------------------------------------

			// retrieve the kong proxy deployment
			deployment, err := c.Client().AppsV1().Deployments(ProxyNamespace).Get(ctx, ProxyDeploymentName, metav1.GetOptions{})
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("proxy deployment %s/%s could not be retrieved: %w", ProxyNamespace, ProxyDeploymentName, err)}
				return
			}

			// check whether the deployment is ready, if not do another cycle
			if deployment.Status.ReadyReplicas < 1 {
				continue
			}

			// -------------------------------------------------------------
			// Admin API Service IP Check
			// -------------------------------------------------------------

			// retrieve the proxy admin service
			adminService, err := c.Client().CoreV1().Services(ProxyNamespace).Get(ctx, ProxyAdminServiceName, metav1.GetOptions{})
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("admin service %s/%s could not be retrieved: %w", ProxyNamespace, ProxyAdminServiceName, err)}
				return
			}

			// validate whether the service has been provisioned an IP yet
			if len(adminService.Status.LoadBalancer.Ingress) < 1 {
				continue
			}
			proxyAdminIP := adminService.Status.LoadBalancer.Ingress[0].IP
			if proxyAdminIP == "" {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("admin service %s/%s unexpectedly had no IP provisioned", ProxyNamespace, ProxyAdminServiceName)}
				return
			}

			// generate the URL to reach the admin api by
			proxyAdminURL, err := url.Parse(fmt.Sprintf("http://%s:%d", proxyAdminIP, ProxyAdminPort))
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("url for admin service %s/%s was invalid: %w", ProxyNamespace, ProxyAdminServiceName, err)}
				return
			}

			// -------------------------------------------------------------
			// Admin API Response Check
			// -------------------------------------------------------------

			// verify that the admin api is accessible
			resp, err := http.Get(fmt.Sprintf("%s/status", proxyAdminURL.String()))
			if err != nil {
				// it can still take a couple seconds for the admin api to be ready
				continue
			}
			defer resp.Body.Close()

			// wait for a 200 OK before proceeding futher
			if resp.StatusCode != http.StatusOK {
				continue
			}

			// read the response body
			b := new(bytes.Buffer)
			_, err = b.ReadFrom(resp.Body)
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("could not read response body from admin api: %w", err)}
				return
			}

			// validate the response body
			body := struct {
				Database struct {
					Reachable bool `json:"reachable"`
				} `json:"database"`
			}{}
			if err := json.Unmarshal(b.Bytes(), &body); err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("could not unmarshal JSON from admin api: %w", err)}
				return
			}

			// ensure that the database is reachable
			if !body.Database.Reachable {
				continue
			}

			// -------------------------------------------------------------
			// Proxy Service IP Check
			// -------------------------------------------------------------

			// retrieve the proxy service
			proxyService, err := c.Client().CoreV1().Services(ProxyNamespace).Get(ctx, ProxyServiceName, metav1.GetOptions{})
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("proxy service %s/%s could not be retrieved: %w", ProxyNamespace, ProxyServiceName, err)}
				return
			}

			// validate whether the service has been provisioned an IP yet
			if len(proxyService.Status.LoadBalancer.Ingress) < 1 {
				continue
			}
			proxyIP := proxyService.Status.LoadBalancer.Ingress[0].IP
			if proxyIP == "" {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("proxy service %s/%s unexpectedly had no IP provisioned", ProxyNamespace, ProxyServiceName)}
				return
			}
			parsedIP := net.ParseIP(proxyIP)
			if parsedIP == nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("proxy service %s/%s IP %s is not valid IPv4 address", ProxyNamespace, ProxyServiceName, proxyIP)}
				return
			}

			// generate the URL to reach the proxy by
			proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", proxyIP, ProxyPort))
			proxyHTTPSURL, err := url.Parse(fmt.Sprintf("https://%s:%d", proxyIP, ProxyHTTPSPort))
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("url for proxy service %s/%s was invalid: %w", ProxyNamespace, ProxyServiceName, err)}
				return
			}

			// -------------------------------------------------------------
			// Proxy Response Check
			// -------------------------------------------------------------

			// verify that the proxy is accessible
			resp, err = http.Get(proxyURL.String())
			if err != nil {
				// it can still take a couple seconds for the proxy to be ready
				continue
			}
			defer resp.Body.Close()

			// wait for a 404 Not Found before proceeding futher
			if resp.StatusCode != http.StatusNotFound {
				continue
			}

			// read the response body
			b = new(bytes.Buffer)
			_, err = b.ReadFrom(resp.Body)
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("could not read response body from proxy url %s: %w", proxyURL, err)}
				return
			}

			// validate the response body
			// {"message":"no Route matched with those values"}
			proxyResponseBody := struct {
				Message string `json:"message"`
			}{}
			if err := json.Unmarshal(b.Bytes(), &proxyResponseBody); err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("could not unmarshal JSON from proxy %s: %w", proxyURL, err)}
				return
			}

			// ensure that the standard 404 Not Found message is being produced by the proxy
			if proxyResponseBody.Message != "no Route matched with those values" {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("received unexpected response from proxy %s: %s", proxyURL, b.String())}
				return
			}

			// -------------------------------------------------------------
			// Proxy UDP Service Check
			//
			// TODO: this is a hack in place to workaround problems in the Kong helm chart when UDP ports are in use:
			//       See: https://github.com/Kong/charts/issues/329
			// -------------------------------------------------------------

			// retrieve the udp service
			udpService, err := c.Client().CoreV1().Services(ProxyNamespace).Get(ctx, ProxyUDPServiceName, metav1.GetOptions{})
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("udp service %s/%s could not be retrieved: %w", ProxyNamespace, ProxyUDPServiceName, err)}
				return
			}

			// validate whether the service has been provisioned an IP yet
			if len(udpService.Status.LoadBalancer.Ingress) < 1 {
				continue
			}
			udpIP := udpService.Status.LoadBalancer.Ingress[0].IP
			if udpIP == "" {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("udp service %s/%s unexpectedly had no IP provisioned", ProxyNamespace, ProxyUDPServiceName)}
				return
			}

			// generate the URL to reach the proxy by
			udpURL, err := url.Parse(fmt.Sprintf("udp://%s:9999", udpIP))
			if err != nil {
				ready <- ProxyReadinessEvent{Err: fmt.Errorf("url for proxy service %s/%s was invalid: %w", ProxyNamespace, ProxyUDPServiceName, err)}
				return
			}

			// -------------------------------------------------------------
			// Report
			// -------------------------------------------------------------

			// all done, report back the endpoints!
			ready <- ProxyReadinessEvent{
				ProxyAdminURL: proxyAdminURL,
				ProxyURL:      proxyURL,
				ProxyHTTPSURL: proxyHTTPSURL,
				ProxyIP:       &parsedIP,
				ProxyUDPUrl:   udpURL,
			}

			return
		}
	}
}
