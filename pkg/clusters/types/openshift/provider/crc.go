package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type CRCProvider struct {
	crc func(ctx context.Context, args ...string) error
}

func NewCRCProvider(ctx context.Context) *CRCProvider {
	return &CRCProvider{
		crc: func(ctx context.Context, args ...string) error {
			stderr := new(bytes.Buffer)
			cmd := exec.CommandContext(ctx, "crc", args...)
			cmd.Stdout = io.Discard
			cmd.Stderr = stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: %w", stderr.String(), err)
			}
			return nil
		},
	}
}

func (p *CRCProvider) CreateCluster(ctx context.Context) error {
	if err := p.setupCRCCluster(ctx); err != nil {
		return err
	}
	return p.startCRCCluster(ctx)
}

func (p *CRCProvider) DeleteCluster(ctx context.Context) error {
	if err := p.stopCRCCluster(ctx); err != nil {
		return err
	}
	return p.deleteCRCCluster(ctx)
}

// -----------------------------------------------------------------------------
// Public Functions - CRC Cluster Management
// -----------------------------------------------------------------------------

// TODO: comment
func (p *CRCProvider) setupCRCCluster(ctx context.Context) error {
	return p.crc(ctx, "setup")
}

// TODO: comment
func (p *CRCProvider) startCRCCluster(ctx context.Context) error {
	return p.crc(ctx, "start")
}

// TODO: comment
func (p *CRCProvider) stopCRCCluster(ctx context.Context) error {
	return p.crc(ctx, "stop")
}

// TODO: comment
func (p *CRCProvider) deleteCRCCluster(ctx context.Context) error {
	return p.crc(ctx, "delete")
}
