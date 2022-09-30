package kind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

var (
	clusterDebug = len(os.Getenv("KTF_KIND_DEBUG")) > 0
)

// Builder generates clusters.Cluster objects backed by Kind given
// provided configuration options.
type Builder struct {
	Name string

	addons         clusters.Addons
	clusterVersion *semver.Version
	configPath     *string
	calicoCNI      bool
	verbosity      int
}

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	return &Builder{
		Name:      uuid.NewString(),
		addons:    make(clusters.Addons),
		verbosity: 3,
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
// See: https://kind.sigs.k8s.io/docs/user/configuration/
func (b *Builder) WithConfig(filename string) *Builder {
	b.configPath = &filename
	return b
}

// WithCalicoCNI disables the default CNI for the kind cluster and instead
// deploys Calico (https://projectcalico.docs.tigera.io/about/about-calico)
// which includes deep features including NetworkPolicy enforcement.
func (b *Builder) WithCalicoCNI() *Builder {
	b.calicoCNI = true
	return b
}

// WithVerbosity sets the kind CLI verbosity level
func (b *Builder) WithVerbosity(v int) *Builder {
	b.verbosity = v
	return b
}

// Build creates and configures clients for a Kind-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (*Cluster, error) {
	deployArgs := make([]string, 0)
	deployArgs = append(deployArgs, fmt.Sprintf("-v%d", b.verbosity))
	if b.clusterVersion != nil {
		deployArgs = append(deployArgs, "--image", "kindest/node:v"+b.clusterVersion.String())
	}

	if b.calicoCNI {
		if err := b.disableDefaultCNI(); err != nil {
			return nil, err
		}

		// if calico is enabled, we can't effectively wait for the cluster to
		// be ready because it wont be possible for it to become ready until we
		// deploy calico, as the default CNI has been disabled.
		deployArgs = append(deployArgs, "--wait", "1s")
	}

	if b.configPath != nil {
		deployArgs = append(deployArgs, "--config", *b.configPath)
	}

	//if clusterDebug {
	deployArgs = append(deployArgs, "--retain")
	//}

	if err := createCluster(ctx, b.Name, deployArgs...); err != nil {
		//if clusterDebug {
		output, tmpErr := os.MkdirTemp(os.TempDir(), "ktf-diag-")
		if err != nil {
			return nil, fmt.Errorf("failed to create cluster %s: %w and failed dumping diagnostics: %s",
				b.Name, err, tmpErr)
		}
		stderr := new(bytes.Buffer)
		cmd := exec.Command("kind", "export", "logs", output)
		cmd.Stdout = io.Discard
		cmd.Stderr = stderr

		if tmpErr := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to create cluster %s: %w and failed dumping diagnostics: %s",
				b.Name, err, tmpErr)
		}
		return nil, fmt.Errorf("failed to create cluster %s: %w, diagnostics in %s", b.Name, err, output)
		//}
		//return nil, fmt.Errorf("failed to create cluster %s: %w", b.Name, err)
	}

	cfg, kc, err := clientForCluster(b.Name)
	if err != nil {
		return nil, err
	}

	cluster := &Cluster{
		name:       b.Name,
		client:     kc,
		cfg:        cfg,
		addons:     make(clusters.Addons),
		deployArgs: deployArgs,
		l:          &sync.RWMutex{},
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
