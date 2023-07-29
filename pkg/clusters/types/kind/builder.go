package kind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// Builder generates clusters.Cluster objects backed by Kind given
// provided configuration options.
type Builder struct {
	Name string

	addons         clusters.Addons
	clusterVersion *semver.Version
	configPath     *string
	configReader   io.Reader
	calicoCNI      bool
	ipv6Only       bool
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:   uuid.NewString(),
		addons: make(clusters.Addons),
	}
}

// WithName indicates a custom name to use for the cluster.
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithClusterVersion configures the Kubernetes cluster version for the Builder
// to use when building the Kind cluster.
func (b *Builder) WithClusterVersion(version semver.Version) *Builder {
	b.clusterVersion = &version
	return b
}

// WithConfig sets a filename containing a KIND config
// See: https://kind.sigs.k8s.io/docs/user/configuration
// This will override any config set previously.
func (b *Builder) WithConfig(filename string) *Builder {
	b.configPath = &filename
	b.configReader = nil
	return b
}

// WithConfigReader sets a reader containing a KIND config
// See: https://kind.sigs.k8s.io/docs/user/configuration
// This will override any config set previously.
func (b *Builder) WithConfigReader(cfg io.Reader) *Builder {
	b.configReader = cfg
	b.configPath = nil
	return b
}

// WithCalicoCNI disables the default CNI for the kind cluster and instead
// deploys Calico (https://projectcalico.docs.tigera.io/about/about-calico)
// which includes deep features including NetworkPolicy enforcement.
func (b *Builder) WithCalicoCNI() *Builder {
	b.calicoCNI = true
	return b
}

// WithIPv6Only configures KIND to only use IPv6.
func (b *Builder) WithIPv6Only() *Builder {
	b.ipv6Only = true
	return b
}

// Build creates and configures clients for a Kind-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	deployArgs := make([]string, 0)
	if b.clusterVersion != nil {
		deployArgs = append(deployArgs, "--image", "kindest/node:v"+b.clusterVersion.String())
	}

	if b.calicoCNI {
		if err := b.disableDefaultCNI(); err != nil {
			return nil, fmt.Errorf("failed disabling default CNI for kind cluster: %w", err)
		}

		// if calico is enabled, we can't effectively wait for the cluster to
		// be ready because it wont be possible for it to become ready until we
		// deploy calico, as the default CNI has been disabled.
		deployArgs = append(deployArgs, "--wait", "1s")
	}

	if b.ipv6Only {
		if err := b.useIPv6Only(); err != nil {
			return nil, fmt.Errorf("failed configuring IPv6-only networking: %w", err)
		}
	}

	var stdin io.Reader
	if b.configPath != nil {
		deployArgs = append(deployArgs, "--config", *b.configPath)
	} else if b.configReader != nil {
		deployArgs = append(deployArgs, "--config", "-")
		stdin = b.configReader
	}

	args := append([]string{"create", "cluster", "--name", b.Name}, deployArgs...)
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create cluster %s: %s: %w", b.Name, stderr.String(), err)
	}

	cfg, kc, err := clientForCluster(b.Name)
	if err != nil {
		return nil, err
	}

	ipFamily := clusters.IPv4
	if b.ipv6Only {
		ipFamily = clusters.IPv6
	}

	cluster := &Cluster{
		name:       b.Name,
		client:     kc,
		cfg:        cfg,
		addons:     make(clusters.Addons),
		deployArgs: deployArgs,
		l:          &sync.RWMutex{},
		ipFamily:   ipFamily,
	}

	if b.calicoCNI {
		if err := clusters.ApplyManifestByURL(ctx, cluster, defaultCalicoManifests); err != nil {
			return nil, err
		}
	}

	if err := utils.ClusterInitHooks(ctx, cluster); err != nil {
		if cleanupErr := cluster.Cleanup(ctx); cleanupErr != nil {
			return nil, fmt.Errorf("multiple errors occurred BUILD_ERROR=(%s) CLEANUP_ERROR=(%s)", err, cleanupErr)
		}
		return nil, err
	}

	return cluster, err
}
