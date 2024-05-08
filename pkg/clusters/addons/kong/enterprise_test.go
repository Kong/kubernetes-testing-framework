package kong

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetLicenseJSON(t *testing.T) {
	t.Logf("saving the original value of %s to restore when tests complete", LicenseDataEnvVar)
	originalValue := os.Getenv(LicenseDataEnvVar)
	defer func() {
		assert.NoError(t, os.Setenv(LicenseDataEnvVar, originalValue))
	}()

	t.Log("gathering various dates to test license validation and expiry")
	now := time.Now().UTC()
	yesterday := now.Add(time.Hour * -24)
	tomorrow := now.Add(time.Hour * 24)

	t.Log("testing license validation for a valid license")
	tomorrowsDateStr := tomorrow.Format(time.RFC3339)
	validLicense := &License{Data: LicenseData{Payload: LicensePayload{ExpirationDate: tomorrowsDateStr}}}
	validLicenseJSON, err := json.Marshal(validLicense)
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(LicenseDataEnvVar, string(validLicenseJSON)))
	parsedLicenseJSON, err := GetLicenseJSONFromEnv()
	assert.NoError(t, err)
	assert.Equal(t, string(validLicenseJSON), parsedLicenseJSON)

	t.Log("testing out of date license validation")
	yesterdaysDateStr := yesterday.Format(time.RFC3339)
	invalidLicense := &License{Data: LicenseData{Payload: LicensePayload{ExpirationDate: yesterdaysDateStr}}}
	invalidLicenseJSON, err := json.Marshal(invalidLicense)
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(LicenseDataEnvVar, string(invalidLicenseJSON)))
	_, err = GetLicenseJSONFromEnv()
	assert.Error(t, err)

	t.Log("testing out of date license validation with a raw string")
	invalidLicense = &License{Data: LicenseData{Payload: LicensePayload{ExpirationDate: "2021-10-20"}}}
	invalidLicenseJSON, err = json.Marshal(invalidLicense)
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(LicenseDataEnvVar, string(invalidLicenseJSON)))
	_, err = GetLicenseJSONFromEnv()
	assert.Error(t, err)
	assert.Equal(t, fmt.Sprintf("the provided %s is expired", LicenseDataEnvVar), err.Error())

	t.Log("testing generation of kubernetes secret license")
	assert.NoError(t, os.Setenv(LicenseDataEnvVar, string(validLicenseJSON)))
	licenseSecret, err := GetLicenseSecretFromEnv()
	assert.NoError(t, err)
	assert.Equal(t, validLicenseJSON, licenseSecret.Data["license"])
}
