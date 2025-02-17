package apiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRequestBodyValidURL(t *testing.T) {
	// Valid case
	req, err := CreateRequestBody("GET", "http://stellar.org")
	assert.NotNil(t, req)
	assert.NoError(t, err)
}

func TestCreateRequestBodyInvalidURL(t *testing.T) {
	invalidURL := "://invalid-url"
	req, err := CreateRequestBody("GET", invalidURL)
	assert.Nil(t, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http GET request creation failed")
}

func TestSetHeadersValidHeaders(t *testing.T) {
	req, err := CreateRequestBody("GET", "http://stellar.org")
	assert.NoError(t, err)

	headers := map[string]interface{}{
		"Content-Type": "application/json",
		"User-Agent":   "GoClient/1.0",
	}
	SetHeaders(req, headers)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "GoClient/1.0", req.Header.Get("User-Agent"))
}

func TestSetHeadersInvalidHeaders(t *testing.T) {
	req, err := CreateRequestBody("GET", "http://stellar.org")
	assert.NoError(t, err)

	headers := map[string]interface{}{
		"Content-Type":  123,               // Invalid: integer
		"User-Agent":    true,              // Invalid: boolean
		"Authorization": "Bearer token123", // Valid: string
	}
	SetHeaders(req, headers)

	// Assertions to check that the invalid headers were skipped
	assert.Equal(t, "", req.Header.Get("Content-Type"))                 // Skipped because it's not a string
	assert.Equal(t, "", req.Header.Get("User-Agent"))                   // Skipped because it's not a string
	assert.Equal(t, "Bearer token123", req.Header.Get("Authorization")) // Set correctly
}

type setAuthHeadersTestCase struct {
	authType       string
	args           map[string]interface{}
	expectedHeader string
	expectedError  string
}

func TestSetAuthHeaders(t *testing.T) {
	testCases := []setAuthHeadersTestCase{
		{
			authType: "basic",
			args: map[string]interface{}{
				"username": "user",
				"password": "pass",
			},
			expectedHeader: "Basic dXNlcjpwYXNz", // base64 of "user:pass"
			expectedError:  "",
		},
		{
			authType: "basic",
			args: map[string]interface{}{
				"password": "pass", // Missing "username"
			},
			expectedHeader: "",
			expectedError:  "missing or invalid username",
		},
		{
			authType: "basic",
			args: map[string]interface{}{
				"username": "user", // Missing "password"
			},
			expectedHeader: "",
			expectedError:  "missing or invalid password",
		},
		{
			authType: "api_key",
			args: map[string]interface{}{
				"api_key": "my-api-key-123",
			},
			expectedHeader: "my-api-key-123",
			expectedError:  "",
		},
		{
			authType:       "api_key",
			args:           map[string]interface{}{}, // Missing "api_key"
			expectedHeader: "",
			expectedError:  "missing or invalid API key",
		},
		{
			authType: "oauth", // Unsupported auth type
			args: map[string]interface{}{
				"username": "user",
				"password": "pass",
			},
			expectedHeader: "",
			expectedError:  "unsupported auth type: oauth",
		},
	}

	// Loop over each test case
	for _, tc := range testCases {
		t.Run(tc.authType, func(t *testing.T) {
			req, err := CreateRequestBody("GET", "http://stellar.org")
			assert.NoError(t, err)

			err = SetAuthHeaders(req, tc.authType, tc.args)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedHeader, req.Header.Get("Authorization"))
			}
		})
	}
}
