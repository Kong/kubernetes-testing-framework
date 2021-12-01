package registry

import (
	"bytes"
	"context"
	"fmt"

	"github.com/blang/semver/v4"
	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	certmanagerclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kong/kubernetes-testing-framework/internal/utils"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/certmanager"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/addons/metallb"
	"github.com/kong/kubernetes-testing-framework/pkg/clusters/types/kind"
	cmutils "github.com/kong/kubernetes-testing-framework/pkg/utils/certmanager"
	dockerutils "github.com/kong/kubernetes-testing-framework/pkg/utils/docker"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/generators"
	"github.com/kong/kubernetes-testing-framework/pkg/utils/kubernetes/networking"
)

// -----------------------------------------------------------------------------
// Registry Addon
// -----------------------------------------------------------------------------

const (
	// AddonName is the unique name of the Kong cluster.Addon
	AddonName clusters.AddonName = "registry"

	// Namespace is the namespace that the Addon compontents
	// will be deployed under when deployment finishes
	Namespace = "registry"
)

// Addon is a Kong Proxy addon which can be deployed on a clusters.Cluster.
type Addon struct {
	name                    string
	registryVersion         *semver.Version
	serviceTypeLoadBalancer bool

	certificatePEM      []byte
	clusterIP           string
	loadBalancerAddress string

	deploymentName  string
	serviceName     string
	certificateName string
	certSecretName  string
	pvcName         string
}

// New produces a new clusters.Addon for Kong but uses a very opionated set of
// default configurations (see the defaults() function for more details).
// If you need to customize your Kong deployment, use the kong.Builder instead.
func New() *Addon {
	return NewBuilder().Build()
}

// Namespace indicates the namespace where the Registry addon components are to be
// deployed and managed.
func (a *Addon) Namespace() string {
	return Namespace
}

// Version indicates the Registry version for this addon.
func (a *Addon) Version() *semver.Version {
	return a.registryVersion
}

// ClusterIP indicates the cluster network address of the registry server
func (a *Addon) ClusterIP() string {
	return a.clusterIP
}

// LoadBalancerAddress indicates the publish network address of the registry
// server this will return empty if the addon was not configured to use a
// LoadBalancer type Service (e.g. builder.WithServiceTypeLoadBalancer()).
func (a *Addon) LoadBalancerAddress() string {
	return a.loadBalancerAddress
}

// Certificate returns the PEM encoded x509 certificate used for TLS
// communications with the registry server.
func (a *Addon) CertificatePEM() []byte {
	return a.certificatePEM
}

// -----------------------------------------------------------------------------
// Registry Addon - Addon Implementation
// -----------------------------------------------------------------------------

func (a *Addon) Name() clusters.AddonName {
	return AddonName
}

func (a *Addon) Dependencies(_ context.Context, cluster clusters.Cluster) []clusters.AddonName {
	// in all cases we depend on cert-manager in order to create the SSL
	// certificate for HTTPS communications to the registry.
	dependencies := []clusters.AddonName{certmanager.AddonName}

	// if we're running on a kind cluster and a loadbalancer service was requested,
	// the metallb is a required dependency. Other cluster implementations are
	// expected to provide their own loadbalancer provisioning facilities.
	if _, ok := cluster.(*kind.Cluster); ok {
		if a.serviceTypeLoadBalancer {
			dependencies = append(dependencies, metallb.AddonName)
		}
	}

	return dependencies
}

const (
	// registryListenPort is the default port the registry container will listen on
	// for HTTP(s) traffic.
	registryListenPort int32 = 5000

	// registryCertDir indicates where the registry certificate will be
	// placed on the nodes so that the container runtime can trust HTTPs connections
	// to the registry server.
	registryCertDir = "/usr/share/ca-certificates"

	// registryCertFilename is the name of the certificate file that will be
	// placed on the nodes so the container runtime can trust HTTPs connections to
	// the registry server.
	registryCertFilename = "ktf-registry.crt"

	// registryCertPath is the entire filesystem path to the registry
	// certificate which will be placed on the nodes so the container runtime can
	// trust HTTPs connections to the registry server.
	registryCertPath = registryCertDir + "/" + registryCertFilename

	// containerdConfigPath is the full path to the containerd configuration file
	containerdConfigPath = "/etc/containerd/config.toml"
)

func (a *Addon) Deploy(ctx context.Context, cluster clusters.Cluster) error {
	// currently this addon can _only_ work on a kind cluster
	if _, ok := cluster.(*kind.Cluster); !ok {
		return fmt.Errorf("the registry addon is currently only supported on kind clusters")
	}

	// wait for dependency addons to be ready first
	if err := clusters.WaitForAddonDependencies(ctx, cluster, a); err != nil {
		return fmt.Errorf("failure waiting for addon dependencies: %w", err)
	}

	// ensure the namespace is created
	if err := clusters.CreateNamespace(ctx, cluster, Namespace); err != nil {
		return fmt.Errorf("could not ensure namespace %s was created for registry addon: %w", Namespace, err)
	}

	// if an specific version was not provided we'll use latest
	registryTag := a.registryVersion.String()
	if a.registryVersion == nil || a.registryVersion.String() == "0.0.0" {
		registryTag = "latest"
	}

	// create a registry container and deployment
	httpbinContainer := corev1.Container{
		Name:  string(AddonName),
		Image: fmt.Sprintf("%s:%s", AddonName, registryTag),
		Ports: []corev1.ContainerPort{
			{
				Name:          "https",
				ContainerPort: registryListenPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}
	deployment := generators.NewDeploymentForContainer(httpbinContainer)
	var err error
	deployment, err = cluster.Client().AppsV1().Deployments(Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create deployment for registry: %w", err)
	}
	a.deploymentName = deployment.Name

	// expose the deployment via Service
	portMapping := map[int32]int32{registryListenPort: 443} //nolint:gomnd
	service := generators.NewServiceForDeploymentWithMappedPorts(deployment, corev1.ServiceTypeClusterIP, portMapping)
	if a.serviceTypeLoadBalancer {
		service = generators.NewServiceForDeploymentWithMappedPorts(deployment, corev1.ServiceTypeLoadBalancer, portMapping)
	}
	service, err = cluster.Client().CoreV1().Services(Namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("could not create service for registry deployment %s: %w", deployment.Name, err)
	}
	a.serviceName = service.Name

	// the loadBalancerAddress is the network address that the registry server can be
	// reached at. By default this is the cluster IP, but if LoadBalancer type
	// service option has been flagged, it will later be the LB address instead.
	loadBalancerAddress := service.Spec.ClusterIP
	a.clusterIP = service.Spec.ClusterIP

	// create a certificate with cert-manager for HTTPS communication to the registry
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registry-cert",
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: "registry-cert-secret",
			DNSNames: []string{
				"registry.registry.svc.cluster.local",
				"registry.registry.svc",
				"registry",
			},
			IssuerRef: cmmeta.ObjectReference{
				Name:  string(certmanager.DefaultIssuerName),
				Kind:  "ClusterIssuer",
				Group: "cert-manager.io",
			},
			IPAddresses: []string{
				service.Spec.ClusterIP,
			},
		},
	}
	a.certificateName = cert.Name

	// if a LoadBalancer type service was requested, wait for the service to
	// be properly provisioned and capture the LB address.
	if a.serviceTypeLoadBalancer {
		// capture the registry address, overriding the previous clusterIP as
		// the main network address for the registry.
		var isIP bool
		loadBalancerAddress, isIP, err = networking.WaitForServiceLoadBalancerAddress(ctx, cluster.Client(), Namespace, service.Name)
		if err != nil {
			return fmt.Errorf("could not retrieve loadbalancer address for registry service: %w", err)
		}
		a.loadBalancerAddress = loadBalancerAddress

		// ensure the LB address is also covered by the cert, whether its an
		// IP address or a Host address.
		if isIP {
			cert.Spec.IPAddresses = append(cert.Spec.IPAddresses, loadBalancerAddress)
		} else {
			cert.Spec.DNSNames = append(cert.Spec.DNSNames, loadBalancerAddress)
		}
	}

	// now that the certificate has been configured, create the certificate
	// object and get the x509 cert for the server generated.
	certSecret, err := cmutils.CreateCertAndWaitForReadiness(ctx, cluster.Config(), Namespace, cert)
	if err != nil {
		return err
	}
	a.certSecretName = certSecret.Name

	// create a persistent volume claim for the repository storage using the default
	// storage provisioner available on the cluster.
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-pvc", AddonName),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
			},
		},
	}
	_, err = cluster.Client().CoreV1().PersistentVolumeClaims(Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create persistent volume claim for registry addon: %w", err)
	}
	a.pvcName = pvc.Name

	// add the storage and certificate to the registry deployment
	deploymentUpdated := false
	for !deploymentUpdated {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context completed while trying to add storage and certs to the registry addone: %w", ctx.Err())
		default:
			// retrieve a fresh copy of the deployment
			deployment, err = cluster.Client().AppsV1().Deployments(Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("could not retrieve deployment for registry: %w", err)
			}

			// add the new certificate and storage volumes to the deployment containers
			deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: certSecret.Name,
						},
					},
				},
				{
					Name: "image-storage",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			}

			// add the new certificate and storage volumemounts to the deployment containers
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "certs",
					MountPath: "/certs",
					ReadOnly:  true,
				},
				{
					Name:      "image-storage",
					MountPath: "/var/lib/registry",
				},
			}

			// update the container's env to configure the certificate location
			deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				{
					Name:  "REGISTRY_HTTP_TLS_CERTIFICATE",
					Value: "/certs/tls.crt",
				},
				{
					Name:  "REGISTRY_HTTP_TLS_KEY",
					Value: "/certs/tls.key",
				},
			}

			// attempt to update the deployment
			deployment, err = cluster.Client().AppsV1().Deployments(Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
			if err != nil {
				if errors.IsConflict(err) {
					continue // something updated the deployment after we retrieved it, try again
				}
				return fmt.Errorf("failed to update registry deployment with storage and certificates: %w", err)
			}
			deploymentUpdated = true
		}
	}

	// gather the decoded certificate from the secret so that we can copy it to
	// the kind container's filesystem.
	crtPEM, ok := certSecret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt missing from registry cert secret %s", certSecret.Name)
	}
	a.certificatePEM = crtPEM

	// write the certificate to a tar archive, as needed for the docker client when
	// copying files to containers.
	containerID := dockerutils.GetKindContainerID(cluster.Name())
	if err := dockerutils.WriteFileToContainer(ctx, containerID, registryCertPath, 0644, crtPEM); err != nil { //nolint:gomnd
		return fmt.Errorf("failed to copy certificate to kind container: %w", err)
	}

	// pull an archive of the containerd directory from the container
	oldContainerdConfig, err := dockerutils.ReadFileFromContainer(ctx, containerID, containerdConfigPath)
	if err != nil {
		return fmt.Errorf("failed to copy containerd configuration from kind container: %w", err)
	}

	// append the new SSL certificate trust configuration to the containerd configuration
	containerdConfig := bytes.NewBuffer(oldContainerdConfig.Bytes())
	certOpts := fmt.Sprintf(`
[plugins."io.containerd.grpc.v1.cri".registry.configs."%s".tls]
  ca_file = "%s"
`, loadBalancerAddress, registryCertPath)
	wc, err := containerdConfig.WriteString(certOpts)
	if err != nil {
		return fmt.Errorf("could not append certificate configuration to containerd config in memory: %w", err)
	}
	if wc != len(certOpts) {
		return fmt.Errorf("wrote %d bytes to containerd configuration in memory, expected %d", wc, len(certOpts))
	}

	// create a new tar archive for the file contents, as required by the docker
	// client api.
	if err := dockerutils.WriteFileToContainer(ctx, containerID, containerdConfigPath, 0644, containerdConfig.Bytes()); err != nil { //nolint:gomnd
		return fmt.Errorf("could not write updated containerd configuration to kind container: %w", err)
	}

	// restart the containerd system service to load the new configuration
	if err := dockerutils.RunPrivilegedCommand(ctx, containerID, "systemctl", "restart", "containerd"); err != nil {
		return fmt.Errorf("failed to restart containerd service after configuration update: %w", err)
	}

	return nil
}

func (a *Addon) Delete(ctx context.Context, cluster clusters.Cluster) error {
	// delete the registry service
	if err := cluster.Client().CoreV1().Services(Namespace).Delete(ctx, a.serviceName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// delete the registry deployment
	if err := cluster.Client().AppsV1().Deployments(Namespace).Delete(ctx, a.deploymentName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// delete the registry pvc
	if err := cluster.Client().CoreV1().PersistentVolumeClaims(Namespace).Delete(ctx, a.pvcName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// generate a certmanager kubernetes typed client
	cmc, err := certmanagerclient.NewForConfig(cluster.Config())
	if err != nil {
		return err
	}

	// delete the registry certificate
	if err := cmc.CertmanagerV1().Certificates(Namespace).Delete(ctx, a.certificateName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// delete the registry certificate secret
	if err := cluster.Client().CoreV1().Secrets(Namespace).Delete(ctx, a.certSecretName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// delete the registry namespace
	if err := cluster.Client().CoreV1().Namespaces().Delete(ctx, Namespace, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (a *Addon) Ready(ctx context.Context, cluster clusters.Cluster) (waitForObjects []runtime.Object, ready bool, err error) {
	return utils.IsNamespaceAvailable(ctx, cluster, Namespace)
}
