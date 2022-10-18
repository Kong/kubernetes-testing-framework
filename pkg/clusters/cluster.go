package clusters

import (
	"context"

	"github.com/blang/semver/v4"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// -----------------------------------------------------------------------------
// Public Types - Cluster Interface
// -----------------------------------------------------------------------------

// Type indicates the type of Kubernetes cluster (e.g. Kind, GKE, e.t.c.)
type Type string

// Cluster objects represent a running Kubernetes cluster.
type Cluster interface {
	// Name indicates the unique name of the running cluster.
	Name() string

	// Type indicates the type of Kubernetes Cluster (e.g. Kind, GKE, e.t.c.)
	Type() Type

	// Version indicates the Kubernetes server version of the cluster.
	Version() (semver.Version, error)

	// Client is the configured *kubernetes.Clientset which can be used to access the Cluster's API
	Client() *kubernetes.Clientset

	// Config provides the *rest.Config for the cluster which is convenient for initiating custom kubernetes.Clientsets.
	Config() *rest.Config

	// Cleanup performance any cleanup and teardown needed to destroy the cluster.
	Cleanup(ctx context.Context) error

	// GetAddon retrieves and Addon object from the cluster if that addon was previously loaded.
	GetAddon(name AddonName) (Addon, error)

	// ListAddons lists the addon components currently loaded into the cluster.
	ListAddons() []Addon

	// DeployAddon deploys a new addon component to the cluster.
	DeployAddon(ctx context.Context, addon Addon) error

	// DeleteAddon removes an existing cluster Addon.
	DeleteAddon(ctx context.Context, addon Addon) error

	// DumpDiagnostics dumps the diagnostic data to temporary directory and return the name
	// of said directory and an error.
	// It uses the provided meta string allow for diagnostics identification.
	DumpDiagnostics(ctx context.Context, meta string) (string, error)
}

type Builder interface {
	Build(ctx context.Context) (Cluster, error)
}
