package gke

import (
	"context"
	"encoding/base64"
	"fmt"

	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/blang/semver/v4"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// -----------------------------------------------------------------------------
// Private Functions - Cluster Management
// -----------------------------------------------------------------------------

// deleteCluster deletes an existing GKE cluster.
func deleteCluster(
	ctx context.Context,
	c *container.ClusterManagerClient,
	name, project, location string,
) (*containerpb.Operation, error) {
	// tear down the cluster and return the teardown operation
	fullname := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, name)
	req := containerpb.DeleteClusterRequest{Name: fullname}
	return c.DeleteCluster(ctx, &req)
}

// clientForCluster provides a *kubernetes.Clientset for a GKE cluster provided the cluster name
// and an oauth token for the gcloud API. This client will only be valid for 1 hour.
func clientForCluster(
	ctx context.Context,
	mgrc *container.ClusterManagerClient,
	oauthToken, name, project, location string,
) (*rest.Config, *kubernetes.Clientset, error) {
	// pull the record of the cluster from the gke API
	fullname := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, name)
	getClusterReq := containerpb.GetClusterRequest{Name: fullname}
	cluster, err := mgrc.GetCluster(ctx, &getClusterReq)
	if err != nil {
		return nil, nil, err
	}

	// decode the TLS data needed to communicate with the cluster securely
	decodedClientCert, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientCertificate)
	if err != nil {
		return nil, nil, err
	}
	decodedClientKey, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClientKey)
	if err != nil {
		return nil, nil, err
	}
	decodedCA, err := base64.StdEncoding.DecodeString(cluster.MasterAuth.ClusterCaCertificate)
	if err != nil {
		return nil, nil, err
	}

	// generate the *rest.Config and kubernetes.Clientset
	cfg := rest.Config{
		BearerToken: oauthToken,
		Host:        "https://" + cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertData: decodedClientCert,
			KeyData:  decodedClientKey,
			CAData:   decodedCA,
		},
	}
	k, err := kubernetes.NewForConfig(&cfg)
	if err != nil {
		return nil, nil, err
	}

	return &cfg, k, nil
}

// listLatestClusterPatchVersions provides a map which provides the semver (and api tag) of the latest
// patch version for any particular major/minor release of Kubernetes on GKE.
func listLatestClusterPatchVersions(ctx context.Context, c *container.ClusterManagerClient, project, location string) (map[string]semver.Version, error) {
	// pull the container server config which includes all the available control plan and node versions
	parent := fmt.Sprintf("projects/%s/locations/%s", project, location)
	req := containerpb.GetServerConfigRequest{Name: parent}
	resp, err := c.GetServerConfig(ctx, &req)
	if err != nil {
		return nil, err
	}
	availableVersions := resp.GetValidMasterVersions()

	// find all valid versions and sort them newest to oldest
	versionMap := make(map[string]semver.Version)
	for _, version := range availableVersions {
		version, err := semver.Parse(version)
		if err != nil {
			return nil, err
		}

		// the google cloud API does not include a filtration option to find only the latest
		// patch versions for any particular major or minor version, so we reduce that ourselves.
		majorMinor := fmt.Sprintf("%d.%d", version.Major, version.Minor)
		if seenVersion, ok := versionMap[majorMinor]; ok {
			if version.LT(seenVersion) {
				continue
			}
		}

		// if we made it here this is the latest patch version for the major/minor, store it.
		versionMap[majorMinor] = version
	}

	return versionMap, nil
}

// clientAuthFromCreds provides a cluster management client and a generated access token, which is everything
// required to create a GKE cluster and then starting accessing its API (assuming the jsonCreds provided refer
// to an IAM user with the necessary permissions, if not an error will be received).
func clientAuthFromCreds(ctx context.Context, jsonCreds []byte) (*container.ClusterManagerClient, string, error) {
	// store the API options with the JSON credentials for auth
	credsOpt := option.WithAuthCredentialsJSON(option.ServiceAccount, jsonCreds)

	// build the google api client to talk to GKE
	mgrc, err := container.NewClusterManagerClient(ctx, credsOpt)
	if err != nil {
		return nil, "", err
	}

	// build the google api IAM client to authenticate to the cluster
	gcreds, err := transport.Creds(ctx, credsOpt, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		return nil, "", err
	}
	oauthToken, err := gcreds.TokenSource.Token()
	if err != nil {
		return nil, "", err
	}

	return mgrc, oauthToken.AccessToken, nil
}
