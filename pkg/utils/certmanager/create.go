package certmanager

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	certmanagerclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CreateCertAndWaitForReadiness creates a given cert-manager certificate on
// the cluster and waits for it to become provisioned as per the given context.
func CreateCertAndWaitForReadiness(
	ctx context.Context,
	cfg *rest.Config,
	namespace string,
	cert *certmanagerv1.Certificate,
) (
	*corev1.Secret,
	error,
) {
	// generate a certmanager kubernetes typed client
	cmc, err := certmanagerclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create a cert-manager API client: %w", err)
	}

	// create the certificate on the cluster
	cert, err = cmc.CertmanagerV1().Certificates(namespace).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate for registry: %w", err)
	}

	// wait for the certificate issuer to provision the cert
	certready := false
	for !certready {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context completed while waiting for registry certificate to provision: %w", ctx.Err())
		default:
			cert, err = cmc.CertmanagerV1().Certificates(namespace).Get(ctx, cert.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve certificate object: %w", err)
			}
			for _, condition := range cert.Status.Conditions {
				// check if the certificate has been issued
				if condition.Reason == certmanagerv1.CertificateRequestReasonIssued &&
					condition.Type == certmanagerv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
					certready = true
					break
				}

				// this extra condition to check if the certificate has been issued exists because it was observed that
				// some versions of cert-manager do no update their status according to the status documentation present
				// in https://cert-manager.io/docs/concepts/certificate/ under "Certificate Lifecycle".
				if condition.Reason == "Ready" &&
					condition.Type == certmanagerv1.CertificateConditionReady && condition.Status == cmmeta.ConditionTrue {
					certready = true
					break
				}
			}
		}
	}

	// generate a core kubernetes typed client
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create a kubernetes client: %w", err)
	}

	// gather the certificate secret
	certSecret, err := k8s.CoreV1().Secrets(namespace).Get(ctx, cert.Spec.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve certificate secret after cert was marked as issued: %w", err)
	}

	return certSecret, nil
}
