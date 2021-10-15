package generators

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
