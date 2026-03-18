package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	// GitHub CLI's public OAuth client ID (device flow, no secret required)
	githubClientID    = "178c6fc778ccc68e1d6a"
	githubDeviceURL   = "https://github.com/login/device/code"
	githubTokenURL    = "https://github.com/login/oauth/access_token"
	githubAPIBase     = "https://api.github.com"
	deviceGrantType   = "urn:ietf:params:oauth:grant-type:device_code"
	requiredScope     = "repo"
)

type GitHubAuth struct{}

func NewGitHubAuth() *GitHubAuth {
	return &GitHubAuth{}
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

func (g *GitHubAuth) Login() (*Token, error) {
	dcr, err := g.requestDeviceCode()
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}

	fmt.Printf("\nPlease visit: %s\n", dcr.VerificationURI)
	fmt.Printf("Enter code:   %s\n\n", dcr.UserCode)

	_ = openBrowser(dcr.VerificationURI)

	fmt.Println("Waiting for authorization...")

	interval := time.Duration(dcr.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		token, err := g.pollForToken(dcr.DeviceCode)
		if err != nil {
			if strings.Contains(err.Error(), "authorization_pending") {
				continue
			}
			if strings.Contains(err.Error(), "slow_down") {
				interval += 5 * time.Second
				continue
			}
			return nil, err
		}

		return token, nil
	}

	return nil, fmt.Errorf("authorization timed out after %d seconds", dcr.ExpiresIn)
}

func (g *GitHubAuth) GetUser(accessToken string) (*User, error) {
	req, err := http.NewRequest("GET", githubAPIBase+"/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Login string `json:"login"`
		Email string `json:"email"`
		ID    int64  `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &User{
		Username: result.Login,
		Email:    result.Email,
		ID:       result.ID,
	}, nil
}

func (g *GitHubAuth) requestDeviceCode() (*deviceCodeResponse, error) {
	data := url.Values{
		"client_id": {githubClientID},
		"scope":     {requiredScope},
	}

	req, err := http.NewRequest("POST", githubDeviceURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, body)
	}

	var dcr deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcr); err != nil {
		return nil, err
	}

	return &dcr, nil
}

func (g *GitHubAuth) pollForToken(deviceCode string) (*Token, error) {
	data := url.Values{
		"client_id":   {githubClientID},
		"device_code": {deviceCode},
		"grant_type":  {deviceGrantType},
	}

	req, err := http.NewRequest("POST", githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != "" {
		return nil, fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}

	return &Token{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		Scope:       result.Scope,
	}, nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}
