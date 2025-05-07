package service

import (
	"hedge/app-services/hedge-admin/models"
	"hedge/common/client"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func InitVaultServiceTest() VaultService {
	vaultService := VaultService{
		service:   mockUtils.AppService,
		appConfig: appConfig,
	}
	return vaultService
}

func createMockVaultSysResponse(progress int, token string, withErrors bool) VaultSysResponse {
	if withErrors {
		return VaultSysResponse{
			Errors: []string{
				"Invalid OTP",
				"Permission denied",
			},
			Nonce:        "abc123-nonce-value",
			OTP:          "123456",
			Progress:     progress,
			EncodedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		}
	}
	return VaultSysResponse{
		Errors: []string{},
		Data: map[string]interface{}{
			"token": token,
		},
		Nonce:        "abc123-nonce-value",
		OTP:          "123456",
		Progress:     progress,
		EncodedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
	}
}

func createMockVaultSecretResponse() VaultSecretResponse {
	mockVaultSecretResponse := VaultSecretResponse{
		RequestId:     "1234-abcd-5678-efgh",
		LeaseId:       "lease-5678",
		Renewable:     true,
		LeaseDuration: 3600,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		WrapInfo:  nil,
		Warnings:  nil,
		Auth:      nil,
		MountType: "kv",
	}
	return mockVaultSecretResponse
}

func Test_SetHttpClient_ClientIsNil(t *testing.T) {
	vaultService := InitVaultServiceTest()
	originalClient := client.Client
	client.Client = nil
	t.Cleanup(func() {
		client.Client = originalClient
	})

	vaultService.SetHttpClient()

	assert.NotNil(t, client.Client)
	httpClient, ok := client.Client.(*http.Client)
	assert.True(t, ok, "Expected client.Client to be of type *http.Client")
	assert.NotNil(t, httpClient.Transport)
	transport, ok := httpClient.Transport.(*http.Transport)
	assert.True(t, ok, "Expected Transport to be of type *http.Transport")
	assert.NotNil(t, transport.TLSClientConfig)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "Expected InsecureSkipVerify to be true")
}

func Test_SetHttpClient_ClientAlreadySet(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	vaultService.SetHttpClient()

	assert.Equal(t, mockedHttpClient, client.Client, "Expected client.Client to remain unchanged")
}

func Test_NewVaultService(t *testing.T) {

	vaultServiceMock := InitVaultServiceTest()

	vaultService := NewVaultService(vaultServiceMock.service, vaultServiceMock.appConfig)

	assert.NotNil(t, vaultService, "Expected NewVaultService to return a non-nil VaultService")
	assert.Equal(t, vaultService.service, vaultServiceMock.service, "Expected VaultService.service to be initialized with the provided service")
	assert.Equal(t, vaultService.appConfig, vaultServiceMock.appConfig, "Expected VaultService.appConfig to be initialized with the provided appConfig")
}

func Test_GenerateRootToken_Passed(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	mockVaultSysResponse := createMockVaultSysResponse(3, "testToken", false)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)
	rootToken, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.NoError(t, err)
	assert.Equal(t, "testToken", rootToken)
}

func Test_GenerateRootToken_Failed_EmptyClient(t *testing.T) {
	vaultService := InitVaultServiceTest()
	client.Client = nil

	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "no such host")
}

func Test_GenerateRootToken_Failed_SignOutputProgressMismatch(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	mockVaultSysResponse := createMockVaultSysResponse(80, "testToken", false)

	client.Client = mockedHttpClient

	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)
	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.EqualError(t, err, "Failed to generate encoded token after signing")
}

func Test_GenerateRootToken_Failed_initOutputHasErrors(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	mockVaultSysResponse := createMockVaultSysResponse(3, "testToken", false)
	mockVaultSysFailedResponse := createMockVaultSysResponse(3, "testToken", true)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysFailedResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)
	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.EqualError(t, err, "error during init response decoding: "+mockVaultSysFailedResponse.Errors[0])
}

func Test_GenerateRootToken_Failed_signOutputHasErrors(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	mockVaultSysResponse := createMockVaultSysResponse(3, "testToken", false)
	mockVaultSysFailedResponse := createMockVaultSysResponse(3, "", true)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysFailedResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)
	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.EqualError(t, err, "Error during root token signing: "+mockVaultSysFailedResponse.Errors[0])
}

func Test_GenerateRootToken_Failed_decodeOutputHasErrors(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	mockVaultSysResponse := createMockVaultSysResponse(3, "testToken", false)
	mockVaultSysFailedResponse := createMockVaultSysResponse(3, "", true)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysFailedResponse, 200, nil)
	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.EqualError(t, err, "error during root token decoding: "+mockVaultSysFailedResponse.Errors[0])
}

func Test_GenerateRootToken_Failed_TokenNotOk(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	mockVaultSysResponse := VaultSysResponse{
		Errors:       []string{},
		Nonce:        "abc123-nonce-value",
		OTP:          "123456",
		Progress:     3,
		EncodedToken: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
	}
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)
	_, err := vaultService.generateRootToken([]string{"key1", "key2", "key3"})

	assert.Error(t, err)
	assert.EqualError(t, err, "decoded token not found in response")
}

func Test_UpdateSecretsInVault_Passed(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	secretName := "vaultconnection"

	mockVaultSecretResponse := createMockVaultSecretResponse()
	mockVaultSysResponse := createMockVaultSysResponse(3, "testToken", false)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/secret/edgex", "GET", mockVaultSecretResponse, 200, nil)
	for _, svc := range HedgeNodeServices {
		mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/secret/edgex"+svc+secretName, "POST", nil, 200, nil)
	}
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/auth/token/revoke", "POST", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)

	err := vaultService.UpdateSecretsInVault(secretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.NoError(t, err)
}

func Test_UpdateSecretsInVault_Failed_OnCoreSetup(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	vaultService.appConfig.NodeType = "CORE"
	secretName := "vaultconnection"
	t.Cleanup(func() {
		vaultService.appConfig.NodeType = "NODE"
	})
	err := vaultService.UpdateSecretsInVault(secretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.Error(t, err)
	assert.EqualError(t, err, "This is a Hedge CORE setup. This secrets update API is meant to run only on NODE")
}

func Test_UpdateSecretsInVault_Failed_ClientIsNil(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = nil
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnection"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	err := vaultService.UpdateSecretsInVault(VaultSecretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "no such host")
}

func Test_UpdateSecretsInVault_Failed_OnGetVaultUnsealKeys(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionerror"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	err := vaultService.UpdateSecretsInVault(VaultSecretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.Error(t, err)
	assert.EqualError(t, err, "Error fetching secret from Vault")
}

func Test_UpdateSecretsInVault_Failed_OnGenerateRootToken(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnection"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	mockVaultSecretResponse := createMockVaultSecretResponse()
	mockVaultSysResponse := createMockVaultSysResponse(1, "testToken", false)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/auth/token/revoke", "POST", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)

	err := vaultService.UpdateSecretsInVault(VaultSecretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.Error(t, err)
	assert.EqualError(t, err, "Failed to generate encoded token after signing")
}

func Test_UpdateSecretsInVault_Failed_EmptyRootToken(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnection"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	mockVaultSecretResponse := createMockVaultSecretResponse()
	mockVaultSysResponse := createMockVaultSysResponse(3, "", false)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "DELETE", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/auth/token/revoke", "POST", mockVaultSecretResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/attempt", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/generate-root/update", "POST", mockVaultSysResponse, 200, nil)
	mockedHttpClient.RegisterExternalMockRestCall("http://edgex-vault:8200/v1/sys/decode-token", "POST", mockVaultSysResponse, 200, nil)

	err := vaultService.UpdateSecretsInVault(VaultSecretName, []models.VaultSecretData{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	})

	assert.Error(t, err)
	assert.EqualError(t, err, "Empty root token generated")
}

func Test_GetSecretsFromVault_Passed(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionOneKeyMap"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	expectedSecrets := map[string]string{"secret_key": "secret_value"}
	secrets, err := vaultService.GetSecretsFromVault(VaultSecretName)

	assert.NoError(t, err)
	assert.Equal(t, expectedSecrets, secrets)
}

func Test_GetSecretsFromVault_Failed_FailsToFetchSecrets(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionerror"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	_, err := vaultService.GetSecretsFromVault(VaultSecretName)

	assert.Error(t, err)
	assert.EqualError(t, err, "Error fetching secret from Vault")
}

func Test_getVaultUnsealKeys_Passed(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionThreeKeysMap"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	unsealKeys, err := vaultService.getVaultUnsealKeys()

	assert.NoError(t, err)
	assert.Equal(t, []string{"unsealKey1", "unsealKey2", "unsealKey3"}, unsealKeys)
}

func Test_getVaultUnsealKeys_Passed_MissingKeys(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient

	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionTwoKeysMap"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	unsealKeys, err := vaultService.getVaultUnsealKeys()

	assert.NoError(t, err)
	assert.Equal(t, []string{"unsealKey1", "", "unsealKey3"}, unsealKeys)
}

func Test_getVaultUnsealKeys_Failed_FailsToFetchSecrets(t *testing.T) {
	mockedHttpClient = utils.NewMockClient()
	vaultService := InitVaultServiceTest()
	client.Client = mockedHttpClient
	originalVaultSecretName := VaultSecretName
	VaultSecretName = "vaultconnectionerror"

	t.Cleanup(func() {
		VaultSecretName = originalVaultSecretName
	})

	_, err := vaultService.getVaultUnsealKeys()

	assert.Error(t, err)
	assert.EqualError(t, err, "Error fetching secret from Vault")
}
