package runbooks

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	k8sgen "github.com/kong/kubernetes-testing-framework/pkg/generators/k8s"
	ktfkind "github.com/kong/kubernetes-testing-framework/pkg/kind"
	ktfmetal "github.com/kong/kubernetes-testing-framework/pkg/metallb"
)

var (
	// ProxyReadyTimeout is the maximum amount of time the tests will wait for the Kong proxy
	// to become available in the cluster before considering the cluster a failure.
	ProxyReadyTimeout = time.Minute * 10
)

// CreateKindClusterWithKongProxy runbook deploys a new kind cluster with the Kong proxy pre-deployed.
// MetalLB will be used to provision the LoadBalancer service for the proxy using the local Docker network for the Kind cluster.
func CreateKindClusterWithKongProxy(ctx context.Context, clusterName string, proxyInformer chan *url.URL, errorInformer chan error) (kc *kubernetes.Clientset, cleanup func(), err error) {
	// configure the cluster cleanup function
	cleanup = func() {
		if v := os.Getenv("KIND_KEEP_CLUSTER"); v == "" { // you can optionally flag the tests to retain the test cluster for inspection.
			ktfkind.DeleteKindCluster(clusterName)
		}
	}

	// setup the kind cluster with the Kong proxy already installed.
	err = ktfkind.CreateKindClusterWithKongProxy(clusterName)
	if err != nil {
		return
	}

	// setup Metallb for the cluster for LoadBalancer addresses for Kong
	if err = ktfmetal.DeployMetallbForKindCluster(clusterName, ktfkind.DefaultKindDockerNetwork); err != nil {
		return
	}

	// retrieve the *kubernetes.Clientset for the cluster
	kc, err = ktfkind.ClientForKindCluster(clusterName)
	if err != nil {
		return
	}

	// get the kong proxy deployment from the cluster
	proxyDeployment, err := getProxyDeployment(ctx, kc)
	if err != nil {
		return
	}

	// start the proxy informer which will send the proxy URL back via channel when it's provisioned in the cluster and expose the service via MetalLB
	proxyLoadBalancerService := k8sgen.NewServiceForDeployment(proxyDeployment, corev1.ServiceTypeLoadBalancer)
	startProxyInformer(ctx, kc, proxyLoadBalancerService, proxyInformer, errorInformer)
	proxyLoadBalancerService, err = kc.CoreV1().Services("kong-system").Create(ctx, proxyLoadBalancerService, metav1.CreateOptions{})
	if err != nil {
		return
	}

	return
}

func getProxyDeployment(ctx context.Context, kc *kubernetes.Clientset) (proxyDeployment *appsv1.Deployment, err error) {
	timeout := time.Now().Add(time.Second * 30)
	for timeout.After(time.Now()) {
		proxyDeployment, err = kc.AppsV1().Deployments("kong-system").Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
		if err != nil {
			continue
		}

		// if we reach here, success!
		err = nil
		break
	}
	return
}

// startProxyInformer creates a goroutine running in the background that will watch for the Kong proxy service to be fully provisioned and will
// subsequently indicate the success by publishing the URL of the Proxy to the provided channel.
func startProxyInformer(ctx context.Context, kc *kubernetes.Clientset, watchService *corev1.Service, readyCh chan *url.URL, errorCh chan error) {
	factory := kubeinformers.NewSharedInformerFactory(kc, ProxyReadyTimeout)
	informer := factory.Core().V1().Services().Informer()
	errors := make([]error, 0)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObject, newObject interface{}) {
			svc, ok := newObject.(*corev1.Service)
			if !ok {
				errors = append(errors, fmt.Errorf("type of %s found", reflect.TypeOf(newObject)))
				return
			}

			if svc.Name == watchService.Name {
				ing := svc.Status.LoadBalancer.Ingress
				if len(ing) > 0 && ing[0].IP != "" {
					for _, port := range svc.Spec.Ports {
						if port.Name == "proxy" {
							u, err := url.Parse(fmt.Sprintf("http://%s:%d", ing[0].IP, port.Port))
							if err != nil {
								errors = append(errors, err)
								return
							}
							readyCh <- u
							close(readyCh)
						}
					}
				}
			}
		},
	})

	go func() {
		informer.Run(ctx.Done())
		if len(errors) > 0 {
			err := errors[0]
			for _, erri := range errors[1:] {
				err = fmt.Errorf("%v: %w", err, erri)
			}
			errorCh <- err
		}
		close(errorCh)
	}()
}
