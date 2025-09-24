package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type VerifyResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// VerifyDeposit sends the SMS body to the external verification API
// Returns true if verified, false otherwise
func VerifyDeposit(body string) (bool, error) {
	url := "https://smsverifierapi-production.up.railway.app/api/verify-deposit"

	payload := map[string]string{"body": body}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	var verifyResp VerifyResponse
	if err := json.Unmarshal(bodyBytes, &verifyResp); err != nil {
		return false, fmt.Errorf("failed to parse response JSON: %v", err)
	}

	// If status is "success", return true
	return verifyResp.Status == "success", nil
}
