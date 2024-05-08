package kind

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/yaml"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Public Functions - Existing Cluster
// -----------------------------------------------------------------------------

// NewFromExisting provides a Cluster object for a given kind cluster by name.
func NewFromExisting(name string) (clusters.Cluster, error) {
	cfg, kc, err := clientForCluster(name)
	if err != nil {
		return nil, err
	}
	return &Cluster{
		name:   name,
		client: kc,
		cfg:    cfg,
		l:      &sync.RWMutex{},
		addons: make(clusters.Addons),
	}, nil
}

// -----------------------------------------------------------------------------
// Private Consts & Vars
// -----------------------------------------------------------------------------

const (
	defaultCalicoManifests = "https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/calico.yaml"
)

// -----------------------------------------------------------------------------
// Private Functions - Cluster Management
// -----------------------------------------------------------------------------

// deleteKindCluster deletes an existing KIND cluster.
func deleteKindCluster(ctx context.Context, name string) error {
	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}

	return nil
}

// clientForCluster provides a *kubernetes.Clientset for a KIND cluster provided the cluster name.
func clientForCluster(name string) (*rest.Config, *kubernetes.Clientset, error) {
	kubeconfig := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", name)
	cmd.Stdout = kubeconfig
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("command %q failed STDERR=(%s): %w", cmd.String(), stderr.String(), err)
	}

	clientCfg, err := clientcmd.NewClientConfigFromBytes(kubeconfig.Bytes())
	if err != nil {
		return nil, nil, err
	}

	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	return cfg, clientset, err
}

const defaultKindConfig = `---
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
`

func (b *Builder) ensureConfigFile() error {
	if b.configPath == nil {
		f, err := os.CreateTemp(os.TempDir(), "ktf-kind-config")
		if err != nil {
			return fmt.Errorf("failed creating temp file for kind config: %w", err)
		}
		defer f.Close()

		_, err = f.WriteString(defaultKindConfig)
		if err != nil {
			return err
		}

		filename := f.Name()
		b.configPath = &filename
	}

	return nil
}

func (b *Builder) disableDefaultCNI() error {
	if err := b.ensureConfigFile(); err != nil {
		return err
	}

	configYAML, err := os.ReadFile(*b.configPath)
	if err != nil {
		return fmt.Errorf("failed reading kind config from %s: %w", *b.configPath, err)
	}

	kindConfig := v1alpha4.Cluster{}
	if err := yaml.Unmarshal(configYAML, &kindConfig); err != nil {
		return fmt.Errorf("failed unmarshalling kind config: %w", err)
	}

	kindConfig.Networking.DisableDefaultCNI = true

	configYAML, err = yaml.Marshal(kindConfig)
	if err != nil {
		return fmt.Errorf("failed marshalling kind config: %w", err)
	}

	err = os.WriteFile(*b.configPath, configYAML, 0o600) //nolint:mnd
	if err != nil {
		return fmt.Errorf("failed writing kind config %s: %w", *b.configPath, err)
	}
	return nil
}

func (b *Builder) useIPv6Only() error {
	if err := b.ensureConfigFile(); err != nil {
		return err
	}

	configYAML, err := os.ReadFile(*b.configPath)
	if err != nil {
		return fmt.Errorf("failed reading kind config from %s: %w", *b.configPath, err)
	}

	kindConfig := v1alpha4.Cluster{}
	if err := yaml.Unmarshal(configYAML, &kindConfig); err != nil {
		return fmt.Errorf("failed unmarshalling kind config: %w", err)
	}

	kindConfig.Networking.IPFamily = v1alpha4.IPv6Family
	// For Windows/OS X Docker compatibility:
	// https://kind.sigs.k8s.io/docs/user/configuration/#ip-family
	kindConfig.Networking.APIServerAddress = "127.0.0.1"

	configYAML, err = yaml.Marshal(kindConfig)
	if err != nil {
		return fmt.Errorf("failed marshalling kind config: %w", err)
	}

	err = os.WriteFile(*b.configPath, configYAML, 0o600) //nolint:mnd
	if err != nil {
		return fmt.Errorf("failed writing kind config %s: %w", *b.configPath, err)
	}
	return nil
}

// exportLogs dumps a kind cluster logs to the specified directory
func exportLogs(ctx context.Context, name string, outDir string) error {
	args := []string{"export", "logs", outDir, "--name", name}

	stderr := new(bytes.Buffer)
	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return nil
}
