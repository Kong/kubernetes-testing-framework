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

func TestLicenseDataUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    LicenseData
		wantErr bool
	}{
		{
			name: "version as string",
			json: `{"payload":{},"signature":"sig","version":"1"}`,
			want: LicenseData{
				Version:   1,
				Signature: "sig",
			},
		},
		{
			name: "version as integer",
			json: `{"payload":{},"signature":"sig","version":1}`,
			want: LicenseData{
				Version:   1,
				Signature: "sig",
			},
		},
		{
			name: "version missing",
			json: `{"payload":{},"signature":"sig"}`,
			want: LicenseData{
				Signature: "sig",
			},
		},
		{
			name:    "version as boolean",
			json:    `{"payload":{},"signature":"sig","version":true}`,
			wantErr: true,
		},
		{
			name: "full license with version as int",
			json: `{"signature":"XXX","version":"1","payload":{"license_expiration_date":"2002-05-20","customer":"tests","license_creation_date":"2002-04-13","support_plan":"None","admin_seats":"1","product_subscription":"Product","license_key":"XXX"}}`,
			want: LicenseData{
				Version: 1,
				Payload: LicensePayload{
					ExpirationDate:      "2002-05-20",
					Customer:            "tests",
					CreationDate:        "2002-04-13",
					SupportPlan:         "None",
					AdminSeats:          "1",
					ProductSubscription: "Product",
					Key:                 "XXX",
				},
				Signature: "XXX",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ld LicenseData
			err := json.Unmarshal([]byte(tt.json), &ld)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, ld)
		})
	}
}
