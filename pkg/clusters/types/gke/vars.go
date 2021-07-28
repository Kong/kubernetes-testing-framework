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
	// GKEClusterType indicates that the Kubernetes cluster was provisioned by Google Kubernetes Engine (GKE)
	GKEClusterType clusters.Type = "gke"

	// waitForClusterTick indicates the number of seconds to wait between cluster checks
	// when deploying a new GKE cluster.
	waitForClusterTick = time.Second * 3
)

var (
	// EnvKeepCluster indicates whether the caller wants the cluster to remain after tests for manual inspection.
	EnvKeepCluster = os.Getenv("GKE_KEEP_CLUSTER")
)
