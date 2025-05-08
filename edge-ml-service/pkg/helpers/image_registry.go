/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"

	"hedge/common/client"
	hedgeErrors "hedge/common/errors"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

const (
	DockerImageRegistryName     = "username"
	DockerImageRegistryPassword = "password"
	DockerImageRegistryKey      = "registryconnection"
)

type ImageRegistryConfigInterface interface {
	LoadRegistryConfig()
	GetRegistryCredentials() RegistryCredentials
	GetImageDigest(imagePath string) (string, hedgeErrors.HedgeError)
}

type ImageRegistryConfig struct {
	service             interfaces.ApplicationService
	RegistryCredentials RegistryCredentials
	httpClient          client.HTTPClient
}

type RegistryCredentials struct {
	RegistryURL string
	UserName    string
	Password    string
}

type TokenData struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	IssuedAt  string `json:"issued_at"`
}

var bearerTokenCache = cache.New(1*time.Minute, 2*time.Minute)

// NewImageRegistryConfig initializes the configuration.
func NewImageRegistryConfig(service interfaces.ApplicationService) ImageRegistryConfigInterface {
	imageRegistryConfig := &ImageRegistryConfig{
		service: service,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // for case when calls to registry take to long
		},
	}
	imageRegistryConfig.LoadRegistryConfig()
	return imageRegistryConfig
}

func (cfg *ImageRegistryConfig) GetRegistryCredentials() RegistryCredentials {
	return cfg.RegistryCredentials
}

// LoadRegistryConfig loads the configurations for the required registries.
func (cfg *ImageRegistryConfig) LoadRegistryConfig() {
	lc := cfg.service.LoggingClient()

	imageRegistryURL, err := cfg.service.GetAppSetting("ImageRegistry")
	if err != nil {
		lc.Errorf("ImageRegistry URL is not configured in Application Settings: %v", err)
	}
	userCredentials, err := cfg.service.SecretProvider().
		GetSecret(DockerImageRegistryKey, DockerImageRegistryName, DockerImageRegistryPassword)
	if err != nil {
		lc.Errorf("Failed to retrieve credentials from secret %s: %v", DockerImageRegistryKey, err)
	}
	if userCredentials[DockerImageRegistryName] == "" ||
		userCredentials[DockerImageRegistryPassword] == "" {
		lc.Errorf(
			"No credentials found for registry key: %s. Skipping this registry.",
			DockerImageRegistryKey,
		)
	}
	cfg.RegistryCredentials.RegistryURL = imageRegistryURL
	cfg.RegistryCredentials.UserName = userCredentials[DockerImageRegistryName]
	cfg.RegistryCredentials.Password = userCredentials[DockerImageRegistryPassword]

	lc.Infof("%s set for pulling images: %s", DockerImageRegistryKey, imageRegistryURL)
}

func (cfg *ImageRegistryConfig) GetImageDigest(imagePath string) (string, hedgeErrors.HedgeError) {
	lc := cfg.service.LoggingClient()

	imagePathParts := strings.Split(imagePath, ":")
	if len(imagePathParts) != 2 {
		errMsg := fmt.Sprintf(
			"Failed to get image digest: invalid imagePath format, expected 'imageName:tag', got: %s",
			imagePath,
		)
		lc.Errorf(errMsg)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errMsg)
	}
	manifestURL, err := cfg.buildManifestURL(imagePathParts[0], imagePathParts[1])
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get image digest: %s", err.Error())
		lc.Errorf(errMsg)
		return "", err
	}
	imageDigest, err := cfg.getImageDigest(manifestURL, "")
	if err != nil {
		errMsg := fmt.Sprintf("Failed to get image digest, URL: %s", manifestURL)
		lc.Errorf(fmt.Sprintf("%s, err %s", errMsg, err.Error()))
		return "", err
	}

	return imageDigest, nil
}

func (cfg *ImageRegistryConfig) buildManifestURL(
	imageName string,
	imageTag string,
) (string, hedgeErrors.HedgeError) {
	registry := cfg.RegistryCredentials.RegistryURL
	registryURLParts := strings.Split(strings.TrimSuffix(registry, "/"), "/")
	if len(registryURLParts) < 2 {
		errMsg := fmt.Sprintf(
			"invalid registryURL format, expected 'registryURL/repository', got: %s",
			registry,
		)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errMsg)
	}
	registryURL, repository := registryURLParts[0], registryURLParts[1]
	return fmt.Sprintf(
		"https://%s/v2/%s/%s/manifests/%s",
		registryURL,
		repository,
		imageName,
		imageTag,
	), nil
}

func (cfg *ImageRegistryConfig) getImageDigest(
	manifestURL string,
	bearerToken string,
) (string, hedgeErrors.HedgeError) {
	lc := cfg.service.LoggingClient()

	creds := cfg.RegistryCredentials
	if creds.RegistryURL == "" || creds.UserName == "" || creds.Password == "" {
		errMsg := "Image registry credentials not set"
		lc.Errorf(errMsg)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, errMsg)
	}

	var response *http.Response
	if bearerToken == "" {
		// Step 1: Make initial HEAD request with Basic Auth
		req, err := http.NewRequest(http.MethodHead, manifestURL, nil)
		if err != nil {
			lc.Errorf(err.Error())
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				"failed to create GET image digest request",
			)
		}
		req.SetBasicAuth(cfg.RegistryCredentials.UserName, cfg.RegistryCredentials.Password)
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

		response, err = cfg.getWithRetry(req, 2)
		if err != nil {
			lc.Errorf(err.Error())
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("failed to make GET image digest request, URL %s", req.URL),
			)
		}
		defer response.Body.Close()

		// Step 2: Handle 401 Unauthorized (expected for most registries) and extract Bearer token URL
		if response.StatusCode == http.StatusUnauthorized {
			authHeader := response.Header.Get("Www-Authenticate")
			if !strings.Contains(authHeader, "Bearer") {
				return "", hedgeErrors.NewCommonHedgeError(
					hedgeErrors.ErrorTypeServerError,
					"authorization failed, GET Bearer token API not found in the response header",
				)
			}
			tokenURL, queryParams, err := extractBearerTokenURL(authHeader)
			if err != nil {
				return "", err
			}

			var token string
			if cachedToken, found := bearerTokenCache.Get(creds.RegistryURL); found && token != "" {
				token = cachedToken.(string)
			} else {
				tokenData, err := cfg.retrieveBearerToken(tokenURL, queryParams)
				if err != nil {
					lc.Errorf(err.Error())
					return "", err
				}
				// Cache the token with expiration
				tokenCacheDuration, err := calculateCacheDuration(tokenData, time.Now())
				if err != nil {
					lc.Errorf("Failed to calculate cache duration: %v", err)
				}
				token = tokenData.Token
				bearerTokenCache.Set(creds.RegistryURL, token, tokenCacheDuration*time.Second)
			}

			// infinite loop here if we don't have permission
			if token != "" {
				return cfg.getImageDigest(manifestURL, token)
			} else {
				errMsg := "error getting bearer token to fetch manifest for the image, check registry configuration"
				lc.Errorf(errMsg)
				return "", hedgeErrors.NewCommonHedgeError(
					hedgeErrors.ErrorTypeServerError,
					errMsg)
			}
		}
	} else {
		// Step 3: Retry the HEAD digest request using Bearer token
		req, err := http.NewRequest(http.MethodHead, manifestURL, nil)
		if err != nil {
			lc.Errorf(err.Error())
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("failed to create retry request, manifest URL %s", manifestURL))
		}
		req.Header.Set("Authorization", "Bearer "+bearerToken)
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		response, err = cfg.httpClient.Do(req)
		if err != nil {
			lc.Errorf(err.Error())
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("error making request with Bearer token, manifest URL %s", manifestURL))
		}
		defer response.Body.Close()
	}

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeNotFound,
				fmt.Sprintf(
					"image not found in registry, status code: %d, manifest URL: %s",
					response.StatusCode,
					manifestURL,
				),
			)
		}
		if response.StatusCode == http.StatusUnauthorized {
			return "", hedgeErrors.NewCommonHedgeError(
				hedgeErrors.ErrorTypeUnauthorized,
				fmt.Sprintf(
					"authorization failed while getting image from registry, status code: %d, manifest URL: %s",
					response.StatusCode,
					manifestURL,
				),
			)
		} else {
			return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("failed to fetch image info, status code: %d, manifest URL: %s", response.StatusCode, manifestURL))
		}
	}

	digest := response.Header.Get("Docker-Content-Digest")
	if digest == "" {
		errMsg := fmt.Sprintf(
			"Docker-Content-Digest header not found in the response, manifest URL %s",
			manifestURL,
		)
		lc.Errorf(errMsg)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	return digest, nil
}

func (cfg *ImageRegistryConfig) retrieveBearerToken(
	tokenURL string,
	queryParams map[string]string,
) (*TokenData, hedgeErrors.HedgeError) {
	lc := cfg.service.LoggingClient()
	errMsgBase := "failed to get Bearer token: "

	queryString := "?"
	for key, value := range queryParams {
		queryString += fmt.Sprintf("%s=%s&", key, value)
	}
	queryString = strings.TrimSuffix(queryString, "&")
	fullTokenURL := tokenURL + queryString
	req, err := http.NewRequest(http.MethodGet, fullTokenURL, nil)
	if err != nil {
		lc.Errorf(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+fmt.Sprintf("failed to create GET token request, URL: %s", fullTokenURL),
		)
	}
	req.SetBasicAuth(cfg.RegistryCredentials.UserName, cfg.RegistryCredentials.Password)

	resp, err := cfg.httpClient.Do(req)
	if err != nil {
		lc.Errorf(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+fmt.Sprintf("error during token retrieval, URL %s", req.URL),
		)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+fmt.Sprintf("failed to retrieve token, status code: %d", resp.StatusCode),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		lc.Errorf(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+fmt.Sprintf("failed to read token response for request URL %s", req.URL),
		)
	}
	var token TokenData
	err = json.Unmarshal(body, &token)
	if err != nil {
		lc.Errorf(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+fmt.Sprintf("failed to parse token response for request URL %s", req.URL),
		)
	}

	return &token, nil
}

func extractBearerTokenURL(authHeader string) (string, map[string]string, hedgeErrors.HedgeError) {
	errMsgBase := "failed to extract Bearer token URL: "
	parts := strings.Split(authHeader, " ")
	if len(parts) < 2 {
		return "", nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+"invalid Www-Authenticate header",
		)
	}

	params := strings.Split(parts[1], ",")
	tokenURL := ""
	queryParams := make(map[string]string)

	for _, param := range params {
		keyValue := strings.SplitN(param, "=", 2)
		if len(keyValue) == 2 {
			key := strings.TrimSpace(keyValue[0])
			value := strings.Trim(keyValue[1], "\"")
			if key == "realm" {
				tokenURL = value
			} else {
				queryParams[key] = value
			}
		}
	}

	if tokenURL == "" {
		return "", nil, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			errMsgBase+"realm (GET JWT token) URL not found in 'Www-Authenticate' header",
		)
	}

	return tokenURL, queryParams, nil
}

func calculateCacheDuration(
	tokenData *TokenData,
	now time.Time,
) (time.Duration, hedgeErrors.HedgeError) {
	issuedAt, err := time.Parse(time.RFC3339Nano, tokenData.IssuedAt)
	if err != nil {
		return 0, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			fmt.Sprintf("failed to parse issued_at: %v", err),
		)
	}
	expirationTime := issuedAt.Add(time.Duration(tokenData.ExpiresIn) * time.Second)

	// Calculate the remaining time until expiration
	cacheDuration := expirationTime.Sub(now)
	if cacheDuration < 0 {
		return 0, hedgeErrors.NewCommonHedgeError(
			hedgeErrors.ErrorTypeServerError,
			"token has already expired",
		)
	}

	return cacheDuration, nil
}

func (cfg *ImageRegistryConfig) getWithRetry(
	req *http.Request,
	maxRetries int,
) (*http.Response, hedgeErrors.HedgeError) {
	var resp *http.Response
	var err error
	for i := 0; i < maxRetries; i++ {
		resp, err = cfg.httpClient.Do(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		fmt.Sprintf("failed after %d retries: %v", maxRetries, err),
	)
}
