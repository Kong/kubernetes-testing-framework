package gke

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/blang/semver/v4"
	"github.com/google/uuid"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// Builder generates clusters.Cluster objects backed by GKE given
// provided configuration options.
type Builder struct {
	Name              string
	project, location string
	jsonCreds         []byte

	createSubnet   bool
	addons         clusters.Addons
	clusterVersion *semver.Version
	majorMinor     string
}

// NewBuilder provides a new *Builder object.
func NewBuilder(gkeJSONCredentials []byte, project, location string) *Builder {
	return &Builder{
		Name:      fmt.Sprintf("t-%s", uuid.NewString()),
		project:   project,
		location:  location,
		jsonCreds: gkeJSONCredentials,
		addons:    make(clusters.Addons),
	}
}

// WithName indicates a custom name to use for the cluster.
func (b *Builder) WithName(name string) *Builder {
	b.Name = name
	return b
}

// WithClusterVersion configures the Kubernetes cluster version for the Builder
// to use when building the GKE cluster.
func (b *Builder) WithClusterVersion(version semver.Version) *Builder {
	b.clusterVersion = &version
	return b
}

// WithClusterMinorVersion configures the Kubernetes cluster version according
// to a provided Major and Minor version, but will automatically select the latest
// patch version of that minor release (for convenience over the caller having to
// know the entire version tag).
func (b *Builder) WithClusterMinorVersion(major, minor uint64) *Builder {
	b.majorMinor = fmt.Sprintf("%d.%d", major, minor)
	return b
}

func (b *Builder) WithCreateSubnet(create bool) *Builder {
	b.createSubnet = create
	return b
}

// Build creates and configures clients for a GKE-based Kubernetes clusters.Cluster.
func (b *Builder) Build(ctx context.Context) (clusters.Cluster, error) {
	// validate the credential contents by finding the IAM service account
	// ID which is creating this cluster.
	var creds map[string]string
	if err := json.Unmarshal(b.jsonCreds, &creds); err != nil {
		return nil, err
	}
	createdByID, ok := creds["client_id"]
	if !ok {
		return nil, fmt.Errorf("provided credentials did not include required 'client_id'")
	}
	if createdByID == "" {
		return nil, fmt.Errorf("provided credentials were invalid: 'client_id' can not be an empty string")
	}
	createdByID = sanitizeCreatedByID(createdByID)

	// generate an auth token and management client
	mgrc, authToken, err := clientAuthFromCreds(ctx, b.jsonCreds)
	if err != nil {
		return nil, err
	}
	defer mgrc.Close()

	// configure the cluster creation request
	parent := fmt.Sprintf("projects/%s/locations/%s", b.project, b.location)
	pbcluster := containerpb.Cluster{
		Name:             b.Name,
		InitialNodeCount: 1,
		// disable the GKE ingress controller, which will otherwise interact with classless Ingresses
		AddonsConfig: &containerpb.AddonsConfig{
			HttpLoadBalancing: &containerpb.HttpLoadBalancing{Disabled: true},
		},
		ResourceLabels: map[string]string{GKECreateLabel: createdByID},
	}
	req := &containerpb.CreateClusterRequest{Parent: parent, Cluster: &pbcluster}

	// use any provided custom cluster version
	if b.clusterVersion != nil && b.majorMinor != "" {
		return nil, fmt.Errorf("options for full cluster version and partial are mutually exclusive")
	}
	if b.clusterVersion != nil {
		pbcluster.InitialClusterVersion = b.clusterVersion.String()
	}
	if b.majorMinor != "" {
		latestPatches, err := listLatestClusterPatchVersions(ctx, mgrc, b.project, b.location)
		if err != nil {
			return nil, err
		}
		v, ok := latestPatches[b.majorMinor]
		if !ok {
			return nil, fmt.Errorf("no available kubernetes version for %s", b.majorMinor)
		}
		pbcluster.InitialClusterVersion = v.String()
	}

	if err := b.createCluster(ctx, req, mgrc, createdByID); err != nil {
		return nil, err
	}

	// wait for cluster readiness
	clusterReady := false
	for !clusterReady {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("failed to build cluster: %w", err)
			}
			return nil, fmt.Errorf("failed to build cluster: context completed")
		default:
			req := containerpb.GetClusterRequest{Name: fmt.Sprintf("%s/clusters/%s", parent, b.Name)}
			cluster, err := mgrc.GetCluster(ctx, &req)
			if err != nil {
				if _, deleteErr := deleteCluster(ctx, mgrc, b.Name, b.project, b.location); deleteErr != nil {
					return nil, fmt.Errorf("failed to retrieve cluster after building (%s), then failed to clean up: %w", err, deleteErr)
				}
				return nil, err
			}
			if cluster.Status == containerpb.Cluster_RUNNING {
				clusterReady = true
				break
			}
			time.Sleep(waitForClusterTick)
		}
	}

	// get the restconfig and kubernetes client for the cluster
	restCFG, k8s, err := clientForCluster(ctx, mgrc, authToken, b.Name, b.project, b.location)
	if err != nil {
		if _, deleteErr := deleteCluster(ctx, mgrc, b.Name, b.project, b.location); deleteErr != nil {
			return nil, fmt.Errorf("failed to get cluster client (%s), then failed to clean up: %w", err, deleteErr)
		}
		return nil, err
	}

	cluster := &Cluster{
		name:      b.Name,
		project:   b.project,
		location:  b.location,
		jsonCreds: b.jsonCreds,
		client:    k8s,
		cfg:       restCFG,
		addons:    make(clusters.Addons),
		l:         &sync.RWMutex{},
	}

	if err := utils.ClusterInitHooks(ctx, cluster); err != nil {
		if cleanupErr := cluster.Cleanup(ctx); cleanupErr != nil {
			return nil, fmt.Errorf("multiple errors occurred BUILD_ERROR=(%s) CLEANUP_ERROR=(%s)", err, cleanupErr)
		}
		return nil, err
	}

	return cluster, nil
}

// createCluster creates the GKE cluster asynchronously.
func (b *Builder) createCluster(ctx context.Context, req *containerpb.CreateClusterRequest, mgrc *container.ClusterManagerClient, createdByID string) error {
	// createSubnet is currently only available via gcloud CLI:
	// https://github.com/googleapis/google-cloud-go/issues/7219
	if b.createSubnet {
		if err := b.createClusterUsingCLI(ctx, req, createdByID); err != nil {
			return err
		}
		return nil
	}

	_, err := mgrc.CreateCluster(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (b *Builder) createClusterUsingCLI(ctx context.Context, req *containerpb.CreateClusterRequest, createdByID string) error {

	cmd := exec.CommandContext(ctx, "gcloud", "container", "clusters", "create", req.Cluster.Name,
		`--project`, b.project,
		`--region`, b.location,
		`--create-subnetwork`, ``,
		`--enable-ip-alias`,
		`--num-nodes`, `1`,
		`--cluster-version`, req.Cluster.InitialClusterVersion,
		`--addons`, `HorizontalPodAutoscaling`,
		`--labels`, fmt.Sprintf(`%s=%s`, GKECreateLabel, createdByID),
		`--async`,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	fmt.Println(cmd.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run gcloud CLI: %w", err)
	}

	return nil
}

func sanitizeCreatedByID(id string) string {
	// Replace dots with dashes as dots are not allowed by the CLI:
	// "It must only contain lowercase letters ([a-z]), numeric characters ([0-9]), underscores (_) and dashes (-)."
	id = strings.ReplaceAll(id, ".", "-")

	// Truncate to the maximum allowed length.
	return id[:63]
}
