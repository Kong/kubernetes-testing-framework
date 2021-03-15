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
	// to become available in the cluster before considering the cluster a failure and panicing. FIXME
	ProxyReadyTimeout = time.Minute * 10
)

// CreateKindClusterWithKongProxy runbook deploys a new kind cluster with the Kong proxy pre-deployed.
// MetalLB will be used to provision the LoadBalancer service for the proxy using the local Docker network for the Kind cluster.
func CreateKindClusterWithKongProxy(ctx context.Context, proxyInformer chan *url.URL, clusterName string) (kc *kubernetes.Clientset, cleanup func(), err error) {
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
	// FIXME - race condition here with image ingress deployment
	proxyDeployment := new(appsv1.Deployment)
	proxyDeployment, err = kc.AppsV1().Deployments("kong-system").Get(ctx, "ingress-controller-kong", metav1.GetOptions{})
	if err != nil {
		return
	}

	// start the proxy informer which will send the proxy URL back via channel when it's provisioned in the cluster and expose the service via MetalLB
	proxyLoadBalancerService := k8sgen.NewServiceForDeployment(proxyDeployment, corev1.ServiceTypeLoadBalancer)
	startProxyInformer(ctx, kc, proxyLoadBalancerService, proxyInformer)
	proxyLoadBalancerService, err = kc.CoreV1().Services("kong-system").Create(ctx, proxyLoadBalancerService, metav1.CreateOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(16)
	}

	return
}

func startProxyInformer(ctx context.Context, kc *kubernetes.Clientset, watchService *corev1.Service, readyCh chan *url.URL) {
	factory := kubeinformers.NewSharedInformerFactory(kc, ProxyReadyTimeout)
	informer := factory.Core().V1().Services().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObject, newObject interface{}) {
			svc, ok := newObject.(*corev1.Service)
			if !ok {
				panic(fmt.Errorf("type of %s found", reflect.TypeOf(newObject))) // FIXME
			}

			if svc.Name == watchService.Name {
				ing := svc.Status.LoadBalancer.Ingress
				if len(ing) > 0 && ing[0].IP != "" {
					// FIXME - need error handling and logging output so this isn't hard to debug later if something breaks it
					for _, port := range svc.Spec.Ports {
						if port.Name == "proxy" {
							u, err := url.Parse(fmt.Sprintf("http://%s:%d", ing[0].IP, port.Port))
							if err != nil {
								panic(err) // FIXME
							}
							readyCh <- u
							close(readyCh)
						}
					}
				}
			}
		},
	})
	go informer.Run(ctx.Done())
}
