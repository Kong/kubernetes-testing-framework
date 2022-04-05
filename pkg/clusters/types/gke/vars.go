package gke

import (
	"os"
	"time"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// GKE Cluster - Vars
// -----------------------------------------------------------------------------

const (
	// GKECreateLabel is the name of the label which be added to any cluster created with KTF and
	// indicates which Google Cloud IAM Service Account created the cluster.
	GKECreateLabel = "ktf_created_by"

	// GKEClusterType indicates that the Kubernetes cluster was provisioned by Google Kubernetes Engine (GKE)
	GKEClusterType clusters.Type = "gke"

	// GKECredsVar indicates the environment variable used to provide auth credentials for GKE
	GKECredsVar = "GOOGLE_APPLICATION_CREDENTIALS" //nolint:gosec

	// GKEProjectVar indicates the environment variable used to provide a default gcloud project
	GKEProjectVar = "GOOGLE_PROJECT"

	// GKELocationVar indicates the environment variable used to provide a default gcloud region
	GKELocationVar = "GOOGLE_LOCATION"

	// waitForClusterTick indicates the number of seconds to wait between cluster checks
	// when deploying a new GKE cluster.
	waitForClusterTick = time.Second * 3
)

var (
	// EnvKeepCluster indicates whether the caller wants the cluster to remain after tests for manual inspection.
	EnvKeepCluster = os.Getenv("GKE_KEEP_CLUSTER")
)
