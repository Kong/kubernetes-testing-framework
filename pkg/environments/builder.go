package environments

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
)

// -----------------------------------------------------------------------------
// Environment Builder
// -----------------------------------------------------------------------------

// Builder is a toolkit for building a new test Environment.
type Builder struct {
	Name string

	addons            clusters.Addons
	existingCluster   clusters.Cluster
	clusterBuilder    clusters.Builder
	kubernetesVersion *semver.Version
	calicoCNI         bool
	ipv6Only          bool
}

// NewBuilder generates a new empty Builder for creating Environments.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to provide the testing environment
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithAddons includes any provided Addon components in the cluster
// after the cluster is deployed.
func (b *Builder) WithAddons(addons ...clusters.Addon) *Builder {
	for _, addon := range addons {
		b.addons[addon.Name()] = addon
	}
	return b
}

// WithExistingCluster causes the resulting environment to re-use an existing
// clusters.Cluster instead of creating a new one.
func (b *Builder) WithExistingCluster(cluster clusters.Cluster) *Builder {
	b.existingCluster = cluster
	return b
}

// WithClusterBuilder instructs the environment builder to use a provided cluster builder instead of the default one.
func (b *Builder) WithClusterBuilder(builder clusters.Builder) *Builder {
	b.clusterBuilder = builder
	return b
}

// WithKubernetesVersion indicates which Kubernetes version to deploy clusters
// with, if the caller wants something other than the default.
func (b *Builder) WithKubernetesVersion(version semver.Version) *Builder {
	b.kubernetesVersion = &version
	return b
}

// WithCalicoCNI indicates that the CNI used for the cluster should be Calico
// as opposed to any other potential default CNI.
func (b *Builder) WithCalicoCNI() *Builder {
	b.calicoCNI = true
	return b
}

// WithIPv6Only configures KIND to only use IPv6.
func (b *Builder) WithIPv6Only() *Builder {
	b.ipv6Only = true
	return b
}

// Build is a blocking call to construct the configured Environment and it's
// underlying Kubernetes cluster. The amount of time that it blocks depends
// entirely on the underlying clusters.Cluster implementation that was requested.
func (b *Builder) Build(ctx context.Context) (env Environment, err error) {
	var cluster clusters.Cluster

	if b.calicoCNI && b.existingCluster != nil {
		return nil, fmt.Errorf("trying to deploy Calico CNI on an existing cluster is not currently supported")
	}

	if b.ipv6Only && b.existingCluster != nil {
		return nil, fmt.Errorf("trying to configure IPv6 only on an existing cluster is not currently supported")
	}

	if b.existingCluster != nil && b.clusterBuilder != nil {
		return nil, fmt.Errorf("Environment cannot specify both existingCluster and clusterBuilder")
	}

	// determine if an existing cluster has been configured for deployment
	if b.existingCluster != nil {
		if b.kubernetesVersion != nil {
			return nil, fmt.Errorf("can't provide kubernetes version when using an existing cluster")
		}
		cluster = b.existingCluster
	} else if b.clusterBuilder != nil {
		if b.kubernetesVersion != nil {
			return nil, fmt.Errorf("can't provide kubernetes version when providing a cluster builder")
		}
		cluster, err = b.clusterBuilder.Build(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		builder := kind.NewBuilder().WithName(b.Name)
		if b.kubernetesVersion != nil {
			builder.WithClusterVersion(*b.kubernetesVersion)
		}
		if b.calicoCNI {
			builder.WithCalicoCNI()
		}
		if b.ipv6Only {
			builder.WithIPv6Only()
		}
		cluster, err = builder.Build(ctx)
		if err != nil {
			return nil, err
		}
	}
	// Ensure that whole cluster is cleaned up if error is returned somewhere from this method.
	defer func() {
		if b.existingCluster == nil && err != nil {
			if errCleanup := cluster.Cleanup(ctx); err != nil {
				err = errors.Join(err, errCleanup)
			}
		}
	}()

	// determine the addon dependencies of the cluster before building
	requiredAddons := make(map[string][]string)
	for _, addon := range b.addons {
		for _, dependency := range addon.Dependencies(ctx, cluster) {
			requiredAddons[string(dependency)] = append(requiredAddons[string(dependency)], string(addon.Name()))
		}
	}

	// verify addon dependency requirements have been met
	requiredAddonsThatAreMissing := make([]string, 0)
	for requiredAddon, neededBy := range requiredAddons {
		found := false
		for _, addon := range b.addons {
			if requiredAddon == string(addon.Name()) {
				found = true
				break
			}
		}
		if !found {
			requiredAddonsThatAreMissing = append(requiredAddonsThatAreMissing, fmt.Sprintf("%s (needed by %s)", requiredAddon, strings.Join(neededBy, ", ")))
		}
	}
	if len(requiredAddonsThatAreMissing) != 0 {
		return nil, fmt.Errorf("addon dependencies were not met, missing: %s", strings.Join(requiredAddonsThatAreMissing, ", "))
	}

	// run each addon deployment asynchronously and collect any errors that occur
	addonDeploymentErrorQueue := make(chan error, len(b.addons))
	for _, addon := range b.addons {
		addonCopy := addon
		go func() {
			if err := cluster.DeployAddon(ctx, addonCopy); err != nil {
				addonDeploymentErrorQueue <- fmt.Errorf("failed to deploy addon %s: %w", addonCopy.Name(), err)
			}
			addonDeploymentErrorQueue <- nil
		}()
	}

	// wait for all deployments to report, and gather up any errors
	collectedDeploymentErrorsCount := 0
	addonDeploymentErrors := make([]error, 0)
	for !(collectedDeploymentErrorsCount == len(b.addons)) {
		if err := <-addonDeploymentErrorQueue; err != nil {
			addonDeploymentErrors = append(addonDeploymentErrors, err)
		}
		collectedDeploymentErrorsCount++
	}

	// if any errors occurred during deployment, report them
	totalFailures := len(addonDeploymentErrors)
	switch totalFailures {
	case 0:
		return &environment{
			name:    b.Name,
			cluster: cluster,
		}, nil
	case 1:
		return nil, addonDeploymentErrors[0]
	default:
		errMsgs := make([]string, 0, totalFailures)
		for _, err := range addonDeploymentErrors {
			errMsgs = append(errMsgs, err.Error())
		}
		return nil, fmt.Errorf("%d addon deployments failed: %s", totalFailures, strings.Join(errMsgs, ", "))
	}
}
