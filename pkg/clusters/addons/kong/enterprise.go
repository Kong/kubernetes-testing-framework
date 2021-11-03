package kong

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// -----------------------------------------------------------------------------
// Kong License - Consts & Vars
// -----------------------------------------------------------------------------

// LicenseDataEnvVar is the environment variable where the Kong enterprise
// license will be stored if available for tests.
const LicenseDataEnvVar = "KONG_LICENSE_DATA"

// partialRFC3339Regex is a regex to match on timestamps that are only the date
// portion of an RFC3339 timestamp, this is commonly used in Kong license timestamps.
var partialRFC3339Regex = regexp.MustCompile("^[0-9]+-[0-9]+-[0-9]+$")

// -----------------------------------------------------------------------------
// Kong License - Public Types
// -----------------------------------------------------------------------------

type LicensePayload struct {
	AdminSeats          string `json:"admin_seats"`
	Customer            string `json:"customer"`
	DataPlanes          string `json:"dataplanes"`
	CreationDate        string `json:"license_creation_date"`
	ExpirationDate      string `json:"license_expiration_date"`
	Key                 string `json:"license_key"`
	ProductSubscription string `json:"product_subscription"`
	SupportPlan         string `json:"support_plan"`
}

type LicenseData struct {
	Payload   LicensePayload `json:"payload"`
	Signature string         `json:"signature"`
	Version   string         `json:"version"`
}

type License struct {
	Data LicenseData `json:"license"`
}

// -----------------------------------------------------------------------------
// Kong License - Helper Functions
// -----------------------------------------------------------------------------

// GetLicenseJSONFromEnv retrieves the license data from the environment and
// validates it, returning the resulting JSON string.
func GetLicenseJSONFromEnv() (string, error) {
	licenseObj, err := GetLicenseFromEnv()
	if err != nil {
		return "", err
	}

	b, err := json.Marshal(licenseObj)
	return string(b), err
}

// GetLicenseFromEnv retrieves the license data from the environment and
// validates it, returning the resulting *License object.
func GetLicenseFromEnv() (*License, error) {
	licenseJSON := os.Getenv(LicenseDataEnvVar)
	if licenseJSON == "" {
		return nil, fmt.Errorf("no license could be found because %s was not set", LicenseDataEnvVar)
	}

	// validate overall structure
	licenseObj := &License{}
	if err := json.Unmarshal([]byte(licenseJSON), licenseObj); err != nil {
		return nil, fmt.Errorf("invalid license JSON: %w", err)
	}

	// validate license expiration date
	expirationDateStr := licenseObj.Data.Payload.ExpirationDate
	if partialRFC3339Regex.MatchString(expirationDateStr) {
		// allow for shorthand dates which don't match the full RFC3339 spec,
		// but assume the very beginning of the day.
		expirationDateStr = fmt.Sprintf("%sT00:00:01Z", expirationDateStr)
	}
	t, err := time.Parse(time.RFC3339, expirationDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid license date (%s): %w", expirationDateStr, err)
	}

	// validate expiration
	if time.Now().UTC().After(t) {
		return nil, fmt.Errorf("the provided %s is expired", LicenseDataEnvVar)
	}

	return licenseObj, nil
}

// GetLicenseSecretYAMLFromEnv retrieves the license data from the environment and
// validates it, returning a Kubernetes Secret manifest containing the license.
func GetLicenseSecretYAMLFromEnv() (string, error) {
	licenseSecret, err := GetLicenseSecretFromEnv()
	if err != nil {
		return "", err
	}

	licenseSecretYAML, err := yaml.Marshal(licenseSecret)
	return string(licenseSecretYAML), err
}

// GetLicenseSecretFromEnv retrieves the license data from the environment and
// validates it, returning a Kubernetes Secret object containing the license.
func GetLicenseSecretFromEnv() (*corev1.Secret, error) {
	// get a validated copy of the license JSON
	licenseJSON, err := GetLicenseJSONFromEnv()
	if err != nil {
		return nil, fmt.Errorf("could not generate license secret: %w", err)
	}

	// generate a secret
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kong-enterprise-license",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"license": []byte(licenseJSON),
		},
	}, nil
}
