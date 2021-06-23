package cluster

import (
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

	// Client is the configured *kubernetes.Clientset which can be used to access the Cluster's API
	Client() *kubernetes.Clientset

	// Config provides the *rest.Config for the cluster which is convenient for initiating custom kubernetes.Clientsets.
	Config() *rest.Config

	// Cleanup performance any cleanup and teardown needed to destroy the cluster.
	Cleanup() error

	// Addons lists the addon components currently loaded into the cluster.
	Addons() []Addon

	// DeployAddon deploys a new addon component to the cluster.
	DeployAddon(addon Addon) error

	// DeleteAddon removes an existing cluster Addon.
	DeleteAddon(addon Addon) error
}

// Addon is a loadable component to extend the functionality of a Cluster.
type Addon interface {
	// Name indicates the unique name of the Addon
	Name() string

	// Deploy deploys the addon component to a provided cluster.
	Deploy(cluster Cluster) error

	// Delete removes the addon componet from the given cluster.
	Delete(cluster Cluster) error
}
