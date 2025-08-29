package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// TokenData represents the structure of the token file.
type TokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

// RefreshRequest is the request structure for refreshing a token.
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshResponse is the response structure for refreshing a token.
type RefreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

// GetTokenFilePath gets the cross-platform token file path.
func GetTokenFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".aws", "sso", "cache", "kiro-auth-token.json"), nil
}

// ReadToken reads and returns the token data from the token file.
func ReadToken() (*TokenData, error) {
	tokenPath, err := GetTokenFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token TokenData
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &token, nil
}

// RefreshToken refreshes the access token using the refresh token.
func RefreshToken() (*TokenData, error) {
	tokenPath, err := GetTokenFilePath()
	if err != nil {
		return nil, err
	}

	// Read the current token
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var currentToken TokenData
	if err := json.Unmarshal(data, &currentToken); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	// Prepare the refresh request
	refreshReq := RefreshRequest{
		RefreshToken: currentToken.RefreshToken,
	}

	reqBody, err := json.Marshal(refreshReq)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Send the refresh request
	resp, err := http.Post(
		"https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to refresh token, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var refreshResp RefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Update the token file
	newToken := TokenData(refreshResp)

	newData, err := json.MarshalIndent(newToken, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize new token: %w", err)
	}

	if err := os.WriteFile(tokenPath, newData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write token file: %w", err)
	}

	return &newToken, nil
}
