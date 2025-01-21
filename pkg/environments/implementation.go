package environments

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// -----------------------------------------------------------------------------
// Test Environment - Implementation
// -----------------------------------------------------------------------------

const (
	objectWaitSleepTime = time.Millisecond * 200

	readyHungDuration = time.Minute * 20

	readyDiagnosticMeta = "WaitForReady"
)

// environment is the default KTF Environment used for testing Kubernetes ingress.
type environment struct {
	name    string
	cluster clusters.Cluster
}

func (env *environment) Name() string {
	return env.name
}

func (env *environment) Cluster() clusters.Cluster {
	return env.cluster
}

func (env *environment) Cleanup(ctx context.Context) error {
	return env.Cluster().Cleanup(ctx)
}

func (env *environment) Ready(ctx context.Context) (waitForObjects []runtime.Object, ready bool, err error) {
	var deployments *appsv1.DeploymentList
	var daemonsets *appsv1.DaemonSetList

	deployments, err = env.Cluster().Client().AppsV1().Deployments("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	daemonsets, err = env.Cluster().Client().AppsV1().DaemonSets("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return
	}

	for i := 0; i < len(deployments.Items); i++ {
		deployment := &(deployments.Items[i])
		if deployment.Status.AvailableReplicas != *deployment.Spec.Replicas {
			waitForObjects = append(waitForObjects, deployment)
		}
	}

	for i := 0; i < len(daemonsets.Items); i++ {
		daemonset := &(daemonsets.Items[i])
		if daemonset.Status.NumberUnavailable > 0 {
			waitForObjects = append(waitForObjects, daemonset)
		}
	}

	for _, addon := range env.Cluster().ListAddons() {
		var waitForAddonObjects []runtime.Object
		waitForAddonObjects, ready, err = addon.Ready(ctx, env.Cluster())
		if err != nil {
			return
		}
		waitForObjects = append(waitForObjects, waitForAddonObjects...)
	}

	ready = len(waitForObjects) == 0
	return
}

func (env *environment) WaitForReady(ctx context.Context) chan error {
	errs := make(chan error)

	go func() {
		// if the cluster fails to become ready after N minutes, assume it's likely stuck and dump a diagnostic bundle.
		// this uses its own timer since we can't catch "go test" timeouts via the ctx.
		hung := time.AfterFunc(readyHungDuration, func() {
			loc, err := env.Cluster().DumpDiagnostics(ctx, readyDiagnosticMeta)
			if err != nil {
				errs <- err
				return
			}
			fmt.Printf("cluster not ready after %s, dumped diag to %s\n", readyHungDuration.String(), loc)
		})
		waitForObjects := make([]runtime.Object, 0)
		for {
			select {
			case <-ctx.Done():
				errs <- fmt.Errorf("context done before environment was ready (remaining objects %+v): %w", waitForObjects, ctx.Err())
				hung.Stop()
				loc, err := env.Cluster().DumpDiagnostics(ctx, readyDiagnosticMeta)
				if err != nil {
					errs <- err
					return
				}
				fmt.Printf("cluster not ready before context completed, dumped diag to %s\n", loc)
				return
			default:
				var ready bool
				var err error
				waitForObjects, ready, err = env.Ready(ctx)
				if err != nil {
					errs <- err
					return
				}
				if ready {
					errs <- nil
					hung.Stop()
					return
				}
				// Wait before retry to prevent spamming env with readiness check.
				select {
				case <-time.After(objectWaitSleepTime):
				case <-ctx.Done():
					hung.Stop()
					return
				}
			}
		}
	}()

	return errs
}
