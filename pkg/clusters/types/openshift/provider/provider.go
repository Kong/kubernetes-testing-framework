package provider

import "context"

type OpenShiftProvider interface {
	CreateCluster(ctx context.Context) error
	DeleteCluster(ctx context.Context) error
}
