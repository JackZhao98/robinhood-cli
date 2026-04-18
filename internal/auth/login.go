package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackzhao/robinhood-cli/internal/config"
)

// Prompter abstracts how we ask for an MFA code when Robinhood demands one,
// and how we surface "approve on phone" status messages while we poll.
// Sheriff approval needs no input — we just poll automatically.
type Prompter interface {
	PromptMFACode(mfaType string) (string, error)
	NotifySheriff(message string)
}

type tokenResponse struct {
	AccessToken          string  `json:"access_token"`
	RefreshToken         string  `json:"refresh_token"`
	TokenType            string  `json:"token_type"`
	ExpiresIn            int     `json:"expires_in"`
	Scope                string  `json:"scope"`
	MFARequired          bool    `json:"mfa_required"`
	MFAType              string  `json:"mfa_type"`
	VerificationWorkflow *struct {
		ID                string `json:"id"`
		WorkflowStatus    string `json:"workflow_status"`
	} `json:"verification_workflow"`
	Detail string `json:"detail"`
}

// Login performs a username/password OAuth flow.
// On success the returned Credentials are persisted via Save().
func Login(username, password string, prompter Prompter) (*Credentials, error) {
	deviceToken, err := LoadOrCreateDeviceToken()
	if err != nil {
		return nil, fmt.Errorf("device token: %w", err)
	}

	form := url.Values{
		"client_id":     {config.ClientID},
		"expires_in":    {"86400"},
		"grant_type":    {"password"},
		"scope":         {"internal"},
		"username":      {username},
		"password":      {password},
		"device_token":  {deviceToken},
		"challenge_type": {"sms"},
		"try_passkeys":  {"false"},
		"token_request_path": {"/login"},
		"create_read_only_secondary_token": {"true"},
		"request_id":    {randomRequestID()},
	}

	resp, err := postOAuth(form, "")
	if err != nil {
		return nil, err
	}

	// Sheriff verification (newer mobile-style approval flow). No human input
	// needed — we just poll until the user taps "approve" on their phone.
	if resp.VerificationWorkflow != nil {
		if err := completeSheriffWorkflow(resp.VerificationWorkflow.ID, deviceToken, prompter); err != nil {
			return nil, err
		}
		resp, err = postOAuth(form, "")
		if err != nil {
			return nil, err
		}
	}

	// Classic MFA challenge (TOTP / SMS 6-digit code) — needs the user to
	// type a code that came in via SMS/authenticator app.
	if resp.MFARequired {
		code, err := prompter.PromptMFACode(resp.MFAType)
		if err != nil {
			return nil, err
		}
		mfaForm := cloneValues(form)
		mfaForm.Set("mfa_code", strings.TrimSpace(code))
		resp, err = postOAuth(mfaForm, "")
		if err != nil {
			return nil, err
		}
	}

	if resp.AccessToken == "" {
		if resp.Detail != "" {
			return nil, fmt.Errorf("login failed: %s", resp.Detail)
		}
		return nil, fmt.Errorf("login failed: no access token returned")
	}

	creds := &Credentials{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		TokenType:    resp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
		DeviceToken:  deviceToken,
	}
	if err := Save(creds); err != nil {
		return nil, fmt.Errorf("save credentials: %w", err)
	}
	return creds, nil
}

// Refresh exchanges the refresh_token for a new access_token.
func Refresh(creds *Credentials) (*Credentials, error) {
	form := url.Values{
		"client_id":     {config.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {creds.RefreshToken},
		"scope":         {"internal"},
		"device_token":  {creds.DeviceToken},
	}
	resp, err := postOAuth(form, "")
	if err != nil {
		return nil, err
	}
	if resp.AccessToken == "" {
		return nil, fmt.Errorf("refresh failed: %s", resp.Detail)
	}
	creds.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		creds.RefreshToken = resp.RefreshToken
	}
	creds.ExpiresAt = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	if err := Save(creds); err != nil {
		return nil, fmt.Errorf("save credentials: %w", err)
	}
	return creds, nil
}

func postOAuth(form url.Values, bearer string) (*tokenResponse, error) {
	req, err := http.NewRequest("POST", config.APIBase+"/oauth2/token/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Robinhood-API-Version", "1.431.4")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parse oauth response (status %d): %s", httpResp.StatusCode, string(body))
	}
	// 4xx without an explicit error field still might mean MFA/workflow.
	if httpResp.StatusCode >= 500 {
		return nil, fmt.Errorf("oauth http %d: %s", httpResp.StatusCode, string(body))
	}
	return &tr, nil
}

func completeSheriffWorkflow(workflowID, deviceToken string, prompter Prompter) error {
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: open a user_machine for the workflow.
	machineID, err := startSheriffMachine(client, workflowID, deviceToken)
	if err != nil {
		return err
	}

	// Step 2: fetch the inquiry to get the sheriff_challenge id (the push prompt).
	challengeID, err := fetchSheriffChallengeID(client, machineID)
	if err != nil {
		return err
	}

	prompter.NotifySheriff("📱 Approve the Robinhood login on your phone — polling for up to 3 minutes...")

	// Step 3: poll until the challenge is validated. No human keypress needed.
	if err := pollSheriffChallenge(client, challengeID, prompter); err != nil {
		return err
	}

	// Step 4: tell the user_machine we're done so the workflow advances.
	return finalizeSheriff(client, machineID)
}

func startSheriffMachine(client *http.Client, workflowID, deviceToken string) (string, error) {
	body := map[string]any{
		"device_id": deviceToken,
		"flow":      "suv",
		"input":     map[string]string{"workflow_id": workflowID},
	}
	bs, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", config.APIBase+"/pathfinder/user_machine/", bytes.NewReader(bs))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("X-Robinhood-API-Version", "1.431.4")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sheriff start: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.ID == "" {
		return "", fmt.Errorf("sheriff start failed (http %d): %s", resp.StatusCode, string(data))
	}
	return parsed.ID, nil
}

func fetchSheriffChallengeID(client *http.Client, machineID string) (string, error) {
	url := fmt.Sprintf("%s/pathfinder/inquiries/%s/user_view/", config.APIBase, machineID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("X-Robinhood-API-Version", "1.431.4")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sheriff inquiry: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	var view struct {
		TypeContext struct {
			Context struct {
				SheriffChallenge struct {
					ID string `json:"id"`
				} `json:"sheriff_challenge"`
			} `json:"context"`
		} `json:"type_context"`
	}
	if err := json.Unmarshal(data, &view); err != nil {
		return "", fmt.Errorf("decode sheriff inquiry: %w (body=%s)", err, string(data))
	}
	if view.TypeContext.Context.SheriffChallenge.ID == "" {
		return "", fmt.Errorf("no sheriff_challenge id in response: %s", string(data))
	}
	return view.TypeContext.Context.SheriffChallenge.ID, nil
}

func pollSheriffChallenge(client *http.Client, challengeID string, prompter Prompter) error {
	deadline := time.Now().Add(3 * time.Minute)
	url := fmt.Sprintf("%s/push/%s/get_prompts_status/", config.APIBase, challengeID)
	heartbeatEvery := 15 * time.Second
	nextHeartbeat := time.Now().Add(heartbeatEvery)

	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", config.UserAgent)
		req.Header.Set("X-Robinhood-API-Version", "1.431.4")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var status struct {
			ChallengeStatus string `json:"challenge_status"`
		}
		_ = json.Unmarshal(data, &status)
		switch strings.ToLower(status.ChallengeStatus) {
		case "validated":
			prompter.NotifySheriff("✅ Approved.")
			return nil
		case "failed", "expired", "denied":
			return fmt.Errorf("sheriff challenge %s", status.ChallengeStatus)
		}
		// Periodic heartbeat so the caller (human or AI) knows we're alive.
		if time.Now().After(nextHeartbeat) {
			remaining := time.Until(deadline).Round(time.Second)
			prompter.NotifySheriff(fmt.Sprintf("   ...still waiting for phone approval (%s left)", remaining))
			nextHeartbeat = time.Now().Add(heartbeatEvery)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("sheriff verification timed out — did you approve on your phone?")
}

func finalizeSheriff(client *http.Client, machineID string) error {
	url := fmt.Sprintf("%s/pathfinder/inquiries/%s/user_view/", config.APIBase, machineID)
	body := map[string]any{
		"sequence":   0,
		"user_input": map[string]string{"status": "continue"},
	}
	bs, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bs))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("X-Robinhood-API-Version", "1.431.4")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sheriff finalize: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sheriff finalize http %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

func cloneValues(in url.Values) url.Values {
	out := url.Values{}
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func randomRequestID() string {
	// Cheap unique-ish id; Robinhood only requires it be present.
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
