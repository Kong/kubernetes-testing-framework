package generators

import (
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/kong/kubernetes-testing-framework/pkg/clusters"
)

// NewKubeConfigForRestConfig provides the bytes for a kubeconfig file for use
// by kubectl or helm given a valid *rest.Config for the target cluster.
func NewKubeConfigForRestConfig(name string, restcfg *rest.Config) ([]byte, error) {
	clientcfg := NewClientConfigForRestConfig(name, restcfg)
	return clientcmd.Write(*clientcfg)
}

// NewClientConfigForRestConfig provides the *clientcmdapi.Config for a cluster
// given a valid *rest.Config for the target cluster.
func NewClientConfigForRestConfig(name string, restcfg *rest.Config) *clientcmdapi.Config {
	// configure the cluster
	cluster := clientcmdapi.NewCluster()
	cluster.CertificateAuthorityData = restcfg.CAData
	cluster.Server = restcfg.Host

	// configure the authdata
	authinfo := clientcmdapi.NewAuthInfo()
	authinfo.AuthProvider = restcfg.AuthProvider
	authinfo.ClientCertificateData = restcfg.CertData
	authinfo.ClientKeyData = restcfg.KeyData
	authinfo.Username = restcfg.Username
	authinfo.Password = restcfg.Password
	authinfo.Token = restcfg.BearerToken

	// configure the current context
	context := clientcmdapi.NewContext()
	context.Cluster = name
	context.AuthInfo = name

	// generate the configuration
	cfg := clientcmdapi.NewConfig()
	cfg.Clusters[name] = cluster
	cfg.Contexts[name] = context
	cfg.AuthInfos[name] = authinfo
	cfg.CurrentContext = name

	return cfg
}

// TempKubeconfig produces a kubeconfig tempfile given a cluster.
// the caller is responsible for cleaning up the file if they want it removed.
func TempKubeconfig(cluster clusters.Cluster) (*os.File, error) {
	// generate a kubeconfig from the cluster rest.Config
	kubeconfigBytes, err := NewKubeConfigForRestConfig(cluster.Name(), cluster.Config())
	if err != nil {
		return nil, err
	}

	// create a tempfile to store the kubeconfig contents
	kubeconfig, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("-kubeconfig-%s", cluster.Name()))
	if err != nil {
		return nil, err
	}

	// write the contents
	c, err := kubeconfig.Write(kubeconfigBytes)
	if err != nil {
		return nil, err
	}

	// validate the file integrity
	if c != len(kubeconfigBytes) {
		return nil, fmt.Errorf("failed to write kubeconfig to %s (only %d/%d written)", kubeconfig.Name(), c, len(kubeconfigBytes))
	}

	return kubeconfig, nil
}
