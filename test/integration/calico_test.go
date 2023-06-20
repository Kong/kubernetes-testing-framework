//go:build integration_tests

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/httpbin"
	"github.com/kong/kubernetes-testing-framework/pkg/environments"
)

func TestKindClusterWithCalicoCNI(t *testing.T) {
	t.Parallel()

	t.Log("configuring the test environment with Calico enabled")
	builder := environments.NewBuilder().WithCalicoCNI().WithAddons(httpbin.New())

	t.Log("building the testing environment and Kubernetes cluster")
	env, err := builder.Build(ctx)
	require.NoError(t, err)

	t.Log("waiting for the testing environment to be ready")
	require.NoError(t, <-env.WaitForReady(ctx))
	defer func() { assert.NoError(t, env.Cleanup(ctx)) }()

	t.Log("verifying that Calico is running on the cluster")
	daemonset, err := env.Cluster().Client().AppsV1().DaemonSets("kube-system").Get(ctx, "calico-node", metav1.GetOptions{})
	require.NoError(t, err)
	require.Greater(t, daemonset.Status.NumberAvailable, int32(0))

	t.Log("collecting the pod IP of the httpbin addon")
	httpbinPodIP := ""
	require.Eventually(t, func() bool {
		pods, err := env.Cluster().Client().CoreV1().Pods(httpbin.DefaultNamespace).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, "httpbin") {
				if pod.Status.PodIP != "" {
					httpbinPodIP = pod.Status.PodIP
					return true
				}
			}
		}
		t.Logf("found %d pods in namespace %s, no httpbin pods were found.", len(pods.Items), httpbin.DefaultNamespace)
		return false
	}, time.Second*30, time.Second)

	t.Log("verifying cluster network connectivity to the httpbin addon")
	httpbinURL := fmt.Sprintf("http://%s/status/200", httpbinPodIP)
	job := generateCURLJob(httpbinURL, 3)
	job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Get(ctx, job.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return job.Status.Succeeded > 0
	}, time.Minute*3, time.Second)

	t.Log("creating a NetworkPolicy to deny traffic to the httpbin addon")
	netpol := generateNetPol(string(httpbin.AddonName), "127.0.0.1/32")
	netpol, err = env.Cluster().Client().NetworkingV1().NetworkPolicies(httpbin.DefaultNamespace).Create(ctx, netpol, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("verifying that traffic to the httpbin addon is denied")
	job = generateCURLJob(httpbinURL, 1)
	job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Get(ctx, job.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return job.Status.Failed > 0
	}, time.Minute, time.Second)

	t.Log("removing the NetworkPolicy to allow traffic to the httpbin addon")
	require.NoError(t, env.Cluster().Client().NetworkingV1().NetworkPolicies(httpbin.DefaultNamespace).Delete(ctx, netpol.Name, metav1.DeleteOptions{}))

	t.Log("verifying that traffic to the httpbin addon now succeeds")
	job = generateCURLJob(httpbinURL, 3)
	job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		job, err = env.Cluster().Client().BatchV1().Jobs(httpbin.DefaultNamespace).Get(ctx, job.Name, metav1.GetOptions{})
		require.NoError(t, err)
		return job.Status.Succeeded > 0
	}, time.Minute, time.Second)
}

func generateCURLJob(url string, retries int32) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
		},
		Spec: batchv1.JobSpec{
			Completions:           pointer.Int32(1),
			ActiveDeadlineSeconds: pointer.Int64(10),
			BackoffLimit:          pointer.Int32(retries),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "curl",
						Image:   "curlimages/curl",
						Command: []string{"curl", "-m", "10", url},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

var tcpproto = corev1.ProtocolTCP

func generateNetPol(app string, allowCIDR string) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.NewString(),
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": app,
				},
			},
			PolicyTypes: []netv1.PolicyType{
				netv1.PolicyTypeIngress,
			},
			Ingress: []netv1.NetworkPolicyIngressRule{{
				From: []netv1.NetworkPolicyPeer{{
					IPBlock: &netv1.IPBlock{
						CIDR: allowCIDR,
					},
				}},
				Ports: []netv1.NetworkPolicyPort{
					{
						Protocol: &tcpproto,
					},
				},
			}},
		},
	}
}
