package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ReadFileFromContainer reads a specific file from a given container by ID.
func ReadFileFromContainer(ctx context.Context, containerID string, path string) (*bytes.Buffer, error) {
	// connect to the local docker environment
	dockerc, err := NewNegotiatedClientWithOpts(ctx, client.FromEnv)
	if err != nil {
		return nil, err
	}

	// pull an archived copy of the path directory from the container
	archiveBuffer, _, err := dockerc.CopyFromContainer(ctx, containerID, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer archiveBuffer.Close()

	// search the archive and verify that the given file by path is present
	archive := tar.NewReader(archiveBuffer)
	for {
		hdr, err := archive.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(path, hdr.Name) {
			data := new(bytes.Buffer)
			if _, err := io.Copy(data, archive); err != nil { //nolint:gosec
				return nil, err
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("could not find file %s in container %s", path, containerID)
}

// WriteFileToContainer writes a specific file to the container given ID.
func WriteFileToContainer(ctx context.Context, containerID string, path string, mode int64, data []byte) error {
	// a tar archive is required by the docker API to send files to the
	// underlying container.
	archiveBuffer := new(bytes.Buffer)
	archive := tar.NewWriter(archiveBuffer)

	// add the header to the archive to identify the filename and mode
	if err := archive.WriteHeader(&tar.Header{
		Name: filepath.Base(path),
		Mode: mode,
		Size: int64(len(data)),
	}); err != nil {
		return err
	}

	// add the file data to the root of the archive
	wc, err := archive.Write(data)
	if err != nil {
		return err
	}
	if wc != len(data) {
		return fmt.Errorf("wrote %d bytes to in-memory tar archive, expected %d", wc, len(data))
	}

	// connect to the local docker environment
	dockerc, err := NewNegotiatedClientWithOpts(ctx, client.FromEnv)
	if err != nil {
		return fmt.Errorf("could not create a client with the local docker system: %w", err)
	}

	// copy the file to the docker container
	return dockerc.CopyToContainer(ctx, containerID, filepath.Dir(path), archiveBuffer, container.CopyToContainerOptions{})
}
