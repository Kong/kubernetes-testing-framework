package eks

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/blang/semver/v4"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	aws_operations "github.com/kong/kubernetes-testing-framework/pkg/clusters/types/eks/aws-operations"
)

// Builder generates clusters.Cluster objects backed by GKE given
// provided configuration options.DeleteVPC
type Builder struct {
	Name string

	addons          clusters.Addons
	clusterVersion  *semver.Version
	nodeMachineType string
	tags            map[string]string
}

const (
	defaultNodeMachineType   = "c5.4xlarge"
	defaultKubernetesVersion = "1.31.1"
)

// NewBuilder provides a new *Builder object.
func NewBuilder() *Builder {
	k8sVer := semver.MustParse(defaultKubernetesVersion)
	return &Builder{
		Name:            fmt.Sprintf("t-%s", uuid.NewString()),
		nodeMachineType: defaultNodeMachineType,
		addons:          make(clusters.Addons),
		clusterVersion:  &k8sVer,
	}
}

// WithName indicates a custom name to use for the cluster.
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithTag add a tag to the cluster to be created
func (b *Builder) WithTag(name, value string) *Builder {
	b.tags[name] = value
	return b
}

// WithClusterVersion configures the Kubernetes cluster version for the Builder
// to use when building the GKE cluster.
func (b *Builder) WithClusterVersion(version semver.Version) *Builder {
	b.clusterVersion = &version
	return b
}

func (b *Builder) WithNodeMachineType(machineType string) *Builder {
	b.nodeMachineType = machineType
	return b
}

// Build creates and configures clients for an EKS-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	err := guardOnEnv()
	if err != nil {
		return nil, err
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AWS SDK config")
	}

	err = aws_operations.CreateEKSClusterAll(ctx, cfg, b.Name, minorVersion(b.clusterVersion), b.nodeMachineType, b.tags)
	if err != nil {
		return nil, err
	}

	// EKS limits the maximum allowed validity of an STS token to 15min (900s)
	return NewFromExisting(ctx, cfg, b.Name)

}

func minorVersion(v *semver.Version) string {
	fullStr := v.String()
	lastIndexOfDot := strings.LastIndex(fullStr, ".")
	if lastIndexOfDot == -1 {
		lastIndexOfDot = 1
	}
	return fullStr[:lastIndexOfDot]
}
