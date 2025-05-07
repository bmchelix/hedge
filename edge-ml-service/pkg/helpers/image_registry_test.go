package helpers

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	svcmocks "hedge/mocks/hedge/common/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errDummy = errors.New("dummy error")

func TestNewImageRegistryConfig(t *testing.T) {
	t.Run("NewImageRegistryConfig - Passed", func(t *testing.T) {
		cfg := NewImageRegistryConfig(u.AppService)

		require.NotNil(t, cfg)
		assert.IsType(t, &ImageRegistryConfig{}, cfg)
		assert.Equal(t, u.AppService, cfg.(*ImageRegistryConfig).service)
		assert.NotNil(t, cfg.(*ImageRegistryConfig).httpClient)
	})
}

func TestImageRegistryConfig_GetRegistryCredentials(t *testing.T) {
	t.Run("GetRegistryCredentials - Passed", func(t *testing.T) {
		expectedCredentials := RegistryCredentials{
			RegistryURL: "testRegistryURL/",
			UserName:    "testuser",
			Password:    "testpassword",
		}
		cfg := &ImageRegistryConfig{
			RegistryCredentials: expectedCredentials,
		}

		credentials := cfg.GetRegistryCredentials()
		require.Equal(t, expectedCredentials, credentials)
	})
}

func TestImageRegistryConfig_LoadRegistryConfig(t *testing.T) {
	t.Run("LoadRegistryConfig - Passed", func(t *testing.T) {
		cfg := &ImageRegistryConfig{
			service: u.AppService,
		}
		cfg.LoadRegistryConfig()

		assert.Equal(t, "test-image-repo/iot/", cfg.RegistryCredentials.RegistryURL)
		assert.Equal(t, "username", cfg.RegistryCredentials.UserName)
		assert.Equal(t, "password", cfg.RegistryCredentials.Password)
	})
}

func TestImageRegistryConfig_BuildManifestURL(t *testing.T) {
	t.Run("buildManifestURL - Passed", func(t *testing.T) {
		cfg := &ImageRegistryConfig{
			RegistryCredentials: RegistryCredentials{
				RegistryURL: "test-image-repo/iot/",
			},
		}

		url, err := cfg.buildManifestURL("test-image-name", "test-tag")
		require.NoError(t, err)
		assert.Equal(t, testGetDigestURL, url)
	})
	t.Run("buildManifestURL - Failed (invalid registry URL format)", func(t *testing.T) {
		cfg := &ImageRegistryConfig{
			RegistryCredentials: RegistryCredentials{
				RegistryURL: "invalid-registry-url/",
			},
		}

		url, err := cfg.buildManifestURL("test-image-name", "test-tag")
		require.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(
			t,
			err.Error(),
			"invalid registryURL format, expected 'registryURL/repository'",
		)
	})
}

func TestImageRegistryConfig_RetrieveBearerToken(t *testing.T) {
	expectedTokenData := &TokenData{
		Token:     "test-bearer-token",
		ExpiresIn: 300,
		IssuedAt:  "111",
	}

	t.Run("retrieveBearerToken - Passed", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		expectedFullURLs := []string{
			"https://auth.example.com/token?service=example-service&scope=read:example",
			"https://auth.example.com/token?scope=read:example&service=example-service",
		}

		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
			Run(func(args mock.Arguments) {
				req := args.Get(0).(*http.Request)
				assert.Condition(t, func() (success bool) {
					for _, expectedURL := range expectedFullURLs {
						if req.URL.String() == expectedURL {
							return true
						}
					}
					return false
				}, "Unexpected URL: %s, expected one of: %v", req.URL.String(), expectedFullURLs)
				assert.Equal(t, "Basic dXNlcm5hbWU6cGFzc3dvcmQ=", req.Header.Get("Authorization"))
			}).
			Return(&http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(
					strings.NewReader(
						fmt.Sprintf(
							`{"token": "%s", "expires_in": %d, "issued_at": "%s"}`,
							expectedTokenData.Token,
							expectedTokenData.ExpiresIn,
							expectedTokenData.IssuedAt,
						),
					),
				),
			}, nil)

		testTokenURL := "https://auth.example.com/token"
		testQueryParams := map[string]string{
			"service": "example-service",
			"scope":   "read:example",
		}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		tokenData, err := cfg.retrieveBearerToken(testTokenURL, testQueryParams)
		require.NoError(t, err)
		assert.Equal(t, expectedTokenData, tokenData)
	})
	t.Run("retrieveBearerToken - Failed (failed to create GET request)", func(t *testing.T) {
		tokenURL := "://invalid-url"
		queryParams := map[string]string{}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			RegistryCredentials: testRegistryCreds,
		}

		tokenData, err := cfg.retrieveBearerToken(tokenURL, queryParams)
		require.Error(t, err)
		assert.Nil(t, tokenData)
		assert.Contains(t, err.Error(), "failed to create GET token request")
	})
	t.Run("retrieveBearerToken - Failed (Error during token retrieval)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(nil, fmt.Errorf("connection error"))

		tokenURL := "https://auth.example.com/token"
		queryParams := map[string]string{}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		tokenData, err := cfg.retrieveBearerToken(tokenURL, queryParams)
		require.Error(t, err)
		assert.Nil(t, tokenData)
		assert.Contains(t, err.Error(), "error during token retrieval")
	})
	t.Run("retrieveBearerToken - Failed (non-OK status code)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"message": "unauthorized"}`)),
		}, nil)

		testTokenURL := "https://auth.example.com/token"
		testQueryParams := map[string]string{}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		tokenData, err := cfg.retrieveBearerToken(testTokenURL, testQueryParams)
		require.Error(t, err)
		assert.Nil(t, tokenData)
		assert.Contains(t, err.Error(), "failed to retrieve token, status code: 401")
	})
	t.Run("retrieveBearerToken - Failed (while parsing response body)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`invalid-json`)),
		}, nil)

		tokenURL := "https://auth.example.com/token"
		queryParams := map[string]string{}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		tokenData, err := cfg.retrieveBearerToken(tokenURL, queryParams)
		require.Error(t, err)
		assert.Nil(t, tokenData)
		assert.Contains(t, err.Error(), "failed to parse token response")
	})
}

func TestImageRegistryConfig_getImageDigest(t *testing.T) {
	t.Run("getImageDigest - Passed", func(t *testing.T) {
		expectedDigest := "sha256:exampledigest"

		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Docker-Content-Digest": []string{expectedDigest}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.getImageDigest(testGetDigestURL, "")
		require.NoError(t, err)
		assert.Equal(t, expectedDigest, digest)
	})
	t.Run(
		"getImageDigest - Passed (handled 401 and retrieves digest with Bearer token)",
		func(t *testing.T) {
			testAuthHeader := `Bearer realm="https://auth.example.com/token",service="example-service",scope="read:example"`
			expectedDigest := testImageDigest
			expectedToken := "example-bearer-token"

			mockHttpClient := &svcmocks.MockHTTPClient{}
			// Initial request returns 401 with Bearer auth header
			mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
				Once().
				Return(&http.Response{
					StatusCode: http.StatusUnauthorized,
					Header:     http.Header{"Www-Authenticate": []string{testAuthHeader}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil)

			// Simulate retrieving Bearer token
			mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
				Once().
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(
						strings.NewReader(fmt.Sprintf(`{"token": "%s"}`, expectedToken)),
					),
				}, nil)

			// Retry request with Bearer token succeeds
			mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
				Once().
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Docker-Content-Digest": []string{expectedDigest}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil)

			cfg := &ImageRegistryConfig{
				service:             u.AppService,
				httpClient:          mockHttpClient,
				RegistryCredentials: testRegistryCreds,
			}

			digest, err := cfg.getImageDigest(testGetDigestURL, "")
			require.NoError(t, err)
			assert.Equal(t, expectedDigest, digest)
		},
	)
	t.Run("getImageDigest - Failed (missing registry credentials)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		cfgNoCreds := &ImageRegistryConfig{
			service:    u.AppService,
			httpClient: mockHttpClient,
			RegistryCredentials: RegistryCredentials{
				RegistryURL: "",
				UserName:    "",
				Password:    "",
			},
		}

		digest, err := cfgNoCreds.getImageDigest(testGetDigestURL, "")

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "Image registry credentials not set")
	})
	t.Run("getImageDigest - Failed (404 response)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.getImageDigest(testGetDigestURL, "")
		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "image not found in registry")
	})
	t.Run("getImageDigest - Failed (server error)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.getImageDigest(testGetDigestURL, "")

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "failed to fetch image info, status code: 500")
	})
	t.Run("getImageDigest - Failed (missing Docker-Content-Digest header)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.getImageDigest(testGetDigestURL, "")
		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "Docker-Content-Digest header not found")
	})
	t.Run("getImageDigest - Failed (failed to create request)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		invalidManifestURL := "://registry.example.com/v2/repository/my-image/manifests/latest"

		digest, err := cfg.getImageDigest(invalidManifestURL, "")

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "failed to create GET image digest request")
	})
	t.Run("getImageDigest - Failed (http request failed)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).
			Return(&http.Response{}, errDummy)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.getImageDigest(testGetDigestURL, "")

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(
			t,
			err.Error(),
			"failed to make GET image digest request, URL https://test-image-repo/v2/iot/test-image-name/manifests/test-tag",
		)
	})
}

func TestImageRegistryConfig_GetImageDigest(t *testing.T) {
	t.Run("GetImageDigest - Passed", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Docker-Content-Digest": []string{testImageDigest}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.GetImageDigest(testImagePath)

		require.NoError(t, err)
		assert.Equal(t, testImageDigest, digest)
	})
	t.Run("GetImageDigest - Failed (invalid image path format)", func(t *testing.T) {
		invalidImagePath := "invalid-image-path"

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.GetImageDigest(invalidImagePath)

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "invalid imagePath format")
	})
	t.Run("GetImageDigest - Failed (build manifest URL failed)", func(t *testing.T) {
		cfg := &ImageRegistryConfig{
			service: u.AppService,
			RegistryCredentials: RegistryCredentials{
				RegistryURL: "invalid-registry-url",
				UserName:    testRegistryCreds.UserName,
				Password:    testRegistryCreds.Password,
			},
		}

		digest, err := cfg.GetImageDigest(testImagePath)

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(
			t,
			err.Error(),
			"invalid registryURL format, expected 'registryURL/repository', got: invalid-registry-url",
		)
	})
	t.Run("GetImageDigest - Failed (failed to retrieve image digest)", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil)

		cfg := &ImageRegistryConfig{
			service:             u.AppService,
			httpClient:          mockHttpClient,
			RegistryCredentials: testRegistryCreds,
		}

		digest, err := cfg.GetImageDigest(testImagePath)

		require.Error(t, err)
		assert.Empty(t, digest)
		assert.Contains(t, err.Error(), "image not found in registry, status code: 404")
	})
}

func TestImageRegistryConfig_getWithRetry(t *testing.T) {
	testReq, _ := http.NewRequest("GET", "https://example.com/test", nil)

	t.Run("getWithRetry - Passed (Success on first attempt)", func(t *testing.T) {
		mockResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("Success")),
		}

		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(mockResp, nil).Once()

		cfg := &ImageRegistryConfig{
			httpClient: mockHttpClient,
		}

		resp, err := cfg.getWithRetry(testReq, 3)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	t.Run("getWithRetry - Passed (Success after retries)", func(t *testing.T) {
		mockResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(strings.NewReader("Success after retries")),
		}

		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(nil, errDummy).Times(2)
		mockHttpClient.On("Do", mock.Anything).Return(mockResp, nil).Once()

		cfg := &ImageRegistryConfig{
			httpClient: mockHttpClient,
		}

		start := time.Now()
		resp, err := cfg.getWithRetry(testReq, 3)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.GreaterOrEqual(t, duration, 2*time.Second)
	})
	t.Run("getWithRetry - Failed", func(t *testing.T) {
		mockHttpClient := &svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(nil, errDummy).Times(2)

		cfg := &ImageRegistryConfig{
			httpClient: mockHttpClient,
		}

		start := time.Now()
		resp, err := cfg.getWithRetry(testReq, 2)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "failed after 2 retries")
		assert.GreaterOrEqual(t, duration, 2*time.Second)
	})
}

func TestExtractBearerTokenURL(t *testing.T) {
	t.Run("extractBearerTokenURL - Passed", func(t *testing.T) {
		testAuthHeader := `Bearer realm="https://auth.example.com/token",service="example-service",scope="read:example"`
		expectedTokenURL := "https://auth.example.com/token"
		expectedParams := map[string]string{
			"service": "example-service",
			"scope":   "read:example",
		}

		tokenURL, params, err := extractBearerTokenURL(testAuthHeader)
		require.NoError(t, err)
		assert.Equal(t, expectedTokenURL, tokenURL)
		assert.Equal(t, expectedParams, params)
	})
	t.Run(
		"extractBearerTokenURL - Passed (handled malformed parameters gracefully)",
		func(t *testing.T) {
			testAuthHeader := `Bearer realm="https://auth.example.com/token",malformed`
			expectedTokenURL := "https://auth.example.com/token"
			expectedParams := map[string]string{}

			tokenURL, params, err := extractBearerTokenURL(testAuthHeader)
			require.NoError(t, err)
			assert.Equal(t, expectedTokenURL, tokenURL)
			assert.Equal(t, expectedParams, params)
		},
	)
	t.Run(
		"extractBearerTokenURL - Failed (invalid Www-Authenticate header format)",
		func(t *testing.T) {
			testAuthHeader := `Bearer`

			tokenURL, params, err := extractBearerTokenURL(testAuthHeader)
			require.Error(t, err)
			assert.Empty(t, tokenURL)
			assert.Nil(t, params)
			assert.Contains(t, err.Error(), "invalid Www-Authenticate header")
		},
	)
	t.Run("extractBearerTokenURL - Failed (realm URL missing)", func(t *testing.T) {
		testAuthHeader := `Bearer service="example-service",scope="read:example"`

		tokenURL, params, err := extractBearerTokenURL(testAuthHeader)
		require.Error(t, err)
		assert.Empty(t, tokenURL)
		assert.Nil(t, params)
		assert.Contains(t, err.Error(), "realm (GET JWT token) URL not found")
	})
	t.Run(
		"extractBearerTokenURL - Successfully extracts token URL without additional parameters",
		func(t *testing.T) {
			testAuthHeader := `Bearer realm="https://auth.example.com/token"`
			expectedTokenURL := "https://auth.example.com/token"
			expectedParams := map[string]string{}

			tokenURL, params, err := extractBearerTokenURL(testAuthHeader)
			require.NoError(t, err)
			assert.Equal(t, expectedTokenURL, tokenURL)
			assert.Equal(t, expectedParams, params)
		},
	)
}

func TestCalculateCacheDuration(t *testing.T) {
	t.Run("calculateCacheDuration - Passed", func(t *testing.T) {
		now := time.Now()
		tokenData := &TokenData{
			IssuedAt:  now.Format(time.RFC3339Nano),
			ExpiresIn: 300, // 5 minutes
		}

		cacheDuration, err := calculateCacheDuration(tokenData, now)
		assert.NoError(t, err)
		assert.Equal(t, 300*time.Second, cacheDuration)
	})
	t.Run("calculateCacheDuration - Failed (Invalid issuedAt)", func(t *testing.T) {
		now := time.Now()
		tokenData := &TokenData{
			IssuedAt:  "invalid-timestamp",
			ExpiresIn: 300,
		}

		cacheDuration, err := calculateCacheDuration(tokenData, now)
		assert.Error(t, err)
		assert.Equal(t, time.Duration(0), cacheDuration)
		assert.Contains(t, err.Error(), "failed to parse issued_at")
	})
	t.Run("calculateCacheDuration - Failed (Expired token)", func(t *testing.T) {
		now := time.Now()
		issuedAt := now.Add(-10 * time.Minute) // Token issued 10 minutes ago
		tokenData := &TokenData{
			IssuedAt:  issuedAt.Format(time.RFC3339Nano),
			ExpiresIn: 300, // 5 minutes
		}

		cacheDuration, err := calculateCacheDuration(tokenData, now)
		assert.Error(t, err)
		assert.Equal(t, time.Duration(0), cacheDuration)
		assert.Contains(t, err.Error(), "token has already expired")
	})
}
