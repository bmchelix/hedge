/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/app-services/hedge-admin/internal/config"
	"hedge/app-services/hedge-admin/models"
	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"net/http"
	"strings"
)

// Vault configuration
const (
	VaultAddr           = "http://edgex-vault:8200"
	GenerateRootAttempt = "/v1/sys/generate-root/attempt"
	UpdateRoot          = "/v1/sys/generate-root/update"
	DecodeToken         = "/v1/sys/decode-token"
	RevokeToken         = "/v1/auth/token/revoke"
	SecretsReadWrite    = "/v1/secret/edgex"
)

var VaultSecretName = "vaultconnection"

var HedgeNodeServices = []string{
	"app-hedge-admin",
	"app-hedge-remediate",
	"app-hedge-device-extensions",
	"app-meta-sync",
	"app-data-enrichment",
	"app-hedge-event-publisher",
	"app-hedge-ml-broker",
	"app-hedge-ml-edge-agent",
}

/*var HedgeCoreServices = []string{
	"app-hedge-user-app-mgmt",
	"app-hedge-ml-management",
	"app-export-biz-data",
	"app-hedge-export",
	"app-hedge-event",
}*/

// VaultSysResponse represents a generic Vault API response structure
type VaultSysResponse struct {
	Errors       []string               `json:"errors,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Nonce        string                 `json:"nonce,omitempty"`
	OTP          string                 `json:"otp,omitempty"`
	Progress     int                    `json:"progress,omitempty"`
	EncodedToken string                 `json:"encoded_token,omitempty"`
}

// VaultSecretResponse represents a generic Vault API response structure
type VaultSecretResponse struct {
	RequestId     string                 `json:"request_id,omitempty"`
	LeaseId       string                 `json:"lease_id,omitempty"`
	Renewable     bool                   `json:"renewable,omitempty"`
	LeaseDuration int                    `json:"lease_duration,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	WrapInfo      interface{}            `json:"wrap_info,omitempty"`
	Warnings      interface{}            `json:"warnings,omitempty"`
	Auth          interface{}            `json:"auth,omitempty"`
	MountType     string                 `json:"mount_type,omitempty"`
}

type VaultService struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
}

func NewVaultService(service interfaces.ApplicationService, appConfig *config.AppConfig) *VaultService {
	vaultService := new(VaultService)
	vaultService.service = service
	vaultService.appConfig = appConfig
	return vaultService
}

func (vaultService VaultService) SetHttpClient() {
	if client.Client == nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Client = &http.Client{Transport: tr}
	}
}

func (vaultService VaultService) UpdateSecretsInVault(secretName string, vaultSecrets []models.VaultSecretData) hedgeErrors.HedgeError {
	lc := vaultService.service.LoggingClient()

	// Stop execution if this is a CORE. This is only meant to be executed on a Hedge NODE
	if strings.Contains(vaultService.appConfig.NodeType, "CORE") {
		errStr := "This is a Hedge CORE setup. This secrets update API is meant to run only on NODE"
		lc.Errorf(errStr)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s", errStr))
	}

	// Get Vault unseal keys from hedge-admin's vault store. This will be used to generate a temp root token
	unsealKeys, hedgeErr := vaultService.getVaultUnsealKeys()
	if hedgeErr != nil {
		return hedgeErr
	}

	rootToken, err := vaultService.generateRootToken(unsealKeys)
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s", err.Error()))
	}
	if len(rootToken) == 0 {
		errStr := "Empty root token generated"
		lc.Error(errStr)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errStr)
	}

	// Fetch and Update the secrets in Vault using the temp root token for all NODE services
	for _, svc := range HedgeNodeServices {
		lc.Infof("Fetch vault secret for %s/%s", svc, secretName)
		// Get existing secrets
		req, err := http.NewRequest("GET", fmt.Sprintf("%s%s%s%s", VaultAddr, SecretsReadWrite, svc, secretName), nil)
		if err != nil {
			lc.Errorf("Error fetch secret request: %s", err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error fetching existing Vault secrets")
		}
		fetchSecretResp, err := client.Client.Do(req)
		if err != nil {
			lc.Errorf("Error fetching vault secret for %s/%s: %s", svc, secretName, err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error fetching existing Vault secrets")
		}
		defer fetchSecretResp.Body.Close()

		var fetchSecretOutput VaultSecretResponse
		if err := json.NewDecoder(fetchSecretResp.Body).Decode(&fetchSecretOutput); err != nil {
			lc.Errorf("Error decoding secret response: %s", err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error decoding existing Vault secrets")
		}

		// Update secrets
		payload := map[string]string{}
		for _, vaultSecret := range vaultSecrets {
			payload[vaultSecret.Key] = vaultSecret.Value
		}
		payloadBuf, _ := json.Marshal(payload)
		req, err = http.NewRequest("POST", fmt.Sprintf("%s%s%s%s", VaultAddr, SecretsReadWrite, svc, secretName), bytes.NewBuffer(payloadBuf))
		if err != nil {
			lc.Errorf("Error updating secret request: %s", err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error updating Vault secrets")
		}
		updateSecretResp, err := client.Client.Do(req)
		if err != nil {
			lc.Errorf("Error updating vault secret %s/%s: %s", svc, secretName, err.Error())
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error updating Vault secrets")
		}
		defer updateSecretResp.Body.Close()
		lc.Infof("Successfully updated vault secret %s/%s", svc, secretName)
	}

	lc.Infof("Successfully updated all vault secrets")

	// Revoke this temp root token
	payload := map[string]string{
		"token": rootToken,
	}
	payloadBuf, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s%s", VaultAddr, RevokeToken), bytes.NewBuffer(payloadBuf))
	if err != nil {
		lc.Errorf("Error revoke token request: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error revoking Vault token")
	}
	req.Header.Add("X-Vault-Token", rootToken)
	revokeResp, err := client.Client.Do(req)
	if err != nil {
		lc.Errorf("Error revoking temp root token: %s", err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error revoking Vault token")
	}
	defer revokeResp.Body.Close()
	lc.Infof("Successfully revoked the temp vault root token")

	return nil
}

func (vaultService VaultService) GetSecretsFromVault(secretName string) (map[string]string, hedgeErrors.HedgeError) {
	// Get Vault unseal keys from hedge-admin's vault store. This will be used to generate a temp root token
	secrets, err := vaultService.service.SecretProvider().GetSecret(secretName)
	if err != nil {
		vaultService.service.LoggingClient().Errorf("Error fetching secret %s from Vault. Err: %v", secretName, err)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Error fetching secret from Vault")
	}
	return secrets, nil
}

func (vaultService VaultService) getVaultUnsealKeys() ([]string, hedgeErrors.HedgeError) {
	vaultKeys, hedgeErr := vaultService.GetSecretsFromVault(VaultSecretName)
	if hedgeErr != nil {
		vaultService.service.LoggingClient().Errorf("Failed fetching the vaultconnection keys from Vault. Err: %s", hedgeErr.Error())
		return nil, hedgeErr
	}
	unsealKeys := []string{vaultKeys["key1"], vaultKeys["key2"], vaultKeys["key3"]}
	return unsealKeys, nil
}

func (vaultService VaultService) generateRootToken(unsealKeys []string) (string, error) {
	lc := vaultService.service.LoggingClient()
	rootToken := ""
	vaultService.SetHttpClient()

	lc.Infof("About to generate a temporary root token")
	// 1. Cancel any ongoing root token generation process
	lc.Infof("Canceling any ongoing root token generation process...")
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s%s", VaultAddr, GenerateRootAttempt), nil)
	if err != nil {
		lc.Errorf("Error creating request: %s", err.Error())
		return rootToken, err
	}
	_, err = client.Client.Do(req)
	if err != nil {
		lc.Errorf("Error cancelling ongoing root generation attempt: %s", err.Error())
		return rootToken, err
	}

	// 2. Initialize the root token generation process
	lc.Infof("Initializing root token generation...")
	req, err = http.NewRequest("POST", fmt.Sprintf("%s%s", VaultAddr, GenerateRootAttempt), nil)
	if err != nil {
		lc.Errorf("Error initializing request: %s", err.Error())
		return rootToken, err
	}
	initResp, err := client.Client.Do(req)
	if err != nil {
		lc.Errorf("Error initializing root token generation: %s", err.Error())
		return rootToken, err
	}
	defer initResp.Body.Close()

	var initOutput VaultSysResponse
	if err := json.NewDecoder(initResp.Body).Decode(&initOutput); err != nil {
		lc.Errorf("Error decoding init response: %s", err.Error())
		return rootToken, err
	}
	if len(initOutput.Errors) > 0 {
		errStr := "error during init response decoding: " + initOutput.Errors[0]
		lc.Errorf(errStr)
		return rootToken, errors.New(errStr)
	}
	nonce, otp := initOutput.Nonce, initOutput.OTP
	lc.Debugf("Nonce received: %s, OTP received: %s", nonce, otp)

	// 3. Sign root token request with the 3 unseal keys
	lc.Infof("Signing root token request with unseal keys...")
	var encodedToken string
	for _, key := range unsealKeys {
		signPayload := map[string]string{"key": key, "nonce": nonce}
		signBody, _ := json.Marshal(signPayload)
		req, err = http.NewRequest("POST", fmt.Sprintf("%s%s", VaultAddr, UpdateRoot), bytes.NewBuffer(signBody))
		if err != nil {
			lc.Errorf("Error initializing request: %s", err.Error())
			return rootToken, err
		}
		req.Header.Add("Content-Type", "application/json")
		signResp, err := client.Client.Do(req)
		if err != nil {
			lc.Errorf("Error signing root token request with key %s: %s", key, err.Error())
			return rootToken, err
		}
		defer signResp.Body.Close()

		var signOutput VaultSysResponse
		if err := json.NewDecoder(signResp.Body).Decode(&signOutput); err != nil {
			lc.Errorf("Error decoding sign response: %s", err.Error())
			return rootToken, err
		}
		if len(signOutput.Errors) > 0 {
			errStr := "Error during root token signing: " + signOutput.Errors[0]
			lc.Errorf(errStr)
			return rootToken, errors.New(errStr)
		}

		lc.Infof("Signing progress: %d/3\n", signOutput.Progress)
		if signOutput.Progress == 3 {
			encodedToken = signOutput.EncodedToken
			break
		}
	}

	if encodedToken == "" {
		errStr := "Failed to generate encoded token after signing"
		lc.Errorf(errStr)
		return rootToken, errors.New(errStr)
	}

	// 4. Decode the generated root token
	lc.Infof("Decoding the generated root token...")
	decodePayload := map[string]string{"otp": otp, "encoded_token": encodedToken}
	decodeBody, _ := json.Marshal(decodePayload)
	req, err = http.NewRequest("POST", fmt.Sprintf("%s%s", VaultAddr, DecodeToken), bytes.NewBuffer(decodeBody))
	if err != nil {
		lc.Errorf("Error initializing request: %s", err.Error())
		return rootToken, err
	}
	req.Header.Add("Content-Type", "application/json")
	decodeResp, err := client.Client.Do(req)
	if err != nil {
		lc.Errorf("Error decoding root token: %s", err.Error())
		return rootToken, err
	}
	defer decodeResp.Body.Close()

	var decodeOutput VaultSysResponse
	if err := json.NewDecoder(decodeResp.Body).Decode(&decodeOutput); err != nil {
		lc.Errorf("error decoding response: %s", err.Error())
		return rootToken, err
	}
	if len(decodeOutput.Errors) > 0 {
		errStr := "error during root token decoding: " + decodeOutput.Errors[0]
		lc.Errorf(errStr)
		return rootToken, errors.New(errStr)
	}

	rootToken, ok := decodeOutput.Data["token"].(string)
	if !ok {
		errStr := "decoded token not found in response"
		lc.Errorf(errStr)
		return rootToken, errors.New(errStr)
	}

	lc.Infof("New temp root token created successfully")
	return rootToken, nil
}
