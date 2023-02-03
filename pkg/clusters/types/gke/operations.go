package gke

import (
	"context"
	"fmt"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
)

// fullOperationName returns a full operation name that is needed when querying its status.
func fullOperationName(project, location, operationName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/operations/%s", project, location, operationName)
}

// waitForOperationDone waits for a given operation to be done. It's going to be aborted when passed context gets cancelled.
func waitForOperationDone(ctx context.Context, mgrc *container.ClusterManagerClient, operationName string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			op, err := mgrc.GetOperation(ctx, &containerpb.GetOperationRequest{Name: operationName})
			if err != nil {
				return err
			}
			if op.Status == containerpb.Operation_DONE {
				return nil
			}
		}
	}
}
