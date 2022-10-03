package kubectl

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// GetKustomizedManifest takes a kustomization and any number of manifest readers. It adds the manifests to the
// kustomization's resources and returns a reader with the rendered kustomization.
func GetKustomizedManifest(kustomization types.Kustomization, manifests ...io.Reader) (io.Reader, error) {
	workDir, err := os.MkdirTemp("", "ktf.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)
	for i, manifest := range manifests {
		orig, err := io.ReadAll(manifest)
		if err != nil {
			return nil, err
		}
		err = os.WriteFile(filepath.Join(workDir, fmt.Sprintf("resource_%d.yaml", i)), orig, 0o600) //nolint:gomnd
		if err != nil {
			return nil, err
		}
		kustomization.Resources = append(kustomization.Resources, fmt.Sprintf("resource_%d.yaml", i))
	}
	marshalled, err := yaml.Marshal(kustomization)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(workDir, "kustomization.yaml"), marshalled, 0o600) //nolint:gomnd
	if err != nil {
		return nil, err
	}
	kustomized, err := runKustomize(workDir)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(kustomized), nil
}

// runKustomize runs kustomize on a path and returns the YAML output.
func runKustomize(path string) ([]byte, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	m, err := k.Run(filesys.MakeFsOnDisk(), path)
	if err != nil {
		return []byte{}, err
	}
	return m.AsYaml()
}
