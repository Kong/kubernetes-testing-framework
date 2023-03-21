package clusters

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// DumpDiagnostics gathers a wide range of generic, diagnostic information from the test cluster,
// to provide a snapshot of it at a given time for offline debugging.
// It uses the provided context and writes the meta string to meta.txt to identify the result set.
// It returns the name of the directory that contains all the produced diagnostics data.
func DumpDiagnostics(ctx context.Context, c Cluster, meta string, outDir string) (string, error) {
	// for each Addon, run the addon diagnostic function
	failedAddons := make(map[string]error)
	for _, addon := range c.ListAddons() {
		diagnostics, err := addon.DumpDiagnostics(ctx, c)
		if err != nil {
			failedAddons[string(addon.Name())] = err
			continue
		}
		if len(diagnostics) > 0 {
			addonOut := filepath.Join(outDir, "addons", string(addon.Name()))
			err = os.MkdirAll(addonOut, 0o750) //nolint:gomnd
			if err != nil {
				failedAddons[string(addon.Name())] = err
				continue
			}
			for filename, content := range diagnostics {
				diagOut, err := os.Create(filepath.Join(addonOut, filename))
				if err != nil {
					failedAddons[string(addon.Name())] = err
					continue
				}
				defer diagOut.Close()
				_, err = diagOut.Write(content)
				if err != nil {
					failedAddons[string(addon.Name())] = err
					continue
				}
			}
		}
	}
	if len(failedAddons) > 0 {
		failedAddonOut, err := os.Create(filepath.Join(outDir, "addon_failures.txt"))
		if err != nil {
			return outDir, err
		}
		defer failedAddonOut.Close()
		for failed, reason := range failedAddons {
			_, err = failedAddonOut.WriteString(fmt.Sprintf("%s: %v\n", failed, reason))
			if err != nil {
				return outDir, err
			}
		}
	}

	// write the diagnostic metadata
	metaOut, err := os.Create(filepath.Join(outDir, "meta.txt"))
	if err != nil {
		return outDir, err
	}
	defer metaOut.Close()
	_, err = metaOut.WriteString(meta)
	if err != nil {
		return outDir, err
	}

	return outDir, nil
}
