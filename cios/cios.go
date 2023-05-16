package cios

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CIOSDeviceClient struct {
	ClientID     string
	AuthURL      string
	MessagingURL string
	privateKey   *rsa.PrivateKey
	HttpClient   *http.Client
	Scope        []string
	accessToken  string
}

func NewProdCIOSDeviceClient(clientID string, scope []string, rsaPemPath string) (*CIOSDeviceClient, error) {
	rsaPem, err := os.ReadFile(rsaPemPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rsa pem: %w", err)
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(rsaPem)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA private key: %w", err)
	}
	return &CIOSDeviceClient{
		ClientID:     clientID,
		AuthURL:      "https://auth.optim.cloud",
		MessagingURL: "https://messaging.optimcloudapis.com/v2",
		privateKey:   privateKey,
		HttpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Scope: scope,
	}, nil
}

type ErrorsResult struct {
	ErrorCode string `json:"error_code"`
	Errors    []struct {
		Field   string `json:"field"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (e ErrorsResult) Error() string {
	msg := fmt.Sprintf("error code: %s", e.ErrorCode)
	for i, err := range e.Errors {
		msg += fmt.Sprintf("error %d: {filed: %s, reason: %s, message: %s}", i, err.Field, err.Reason, err.Message)
	}
	return msg
}

func (c *CIOSDeviceClient) prepReq(req *http.Request) error {
	if c.shouldTokenUpdate() {
		err := c.UpdateAccessToken()
		if err != nil {
			return err
		}
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	return nil
}

func (c *CIOSDeviceClient) Do(req *http.Request) (*http.Response, error) {
	err := c.prepReq(req)
	if err != nil {
		return nil, err
	}

	return c.HttpClient.Do(req)
}

func (c *CIOSDeviceClient) PublishMessage(channelID string, msg []byte) error {
	msgPubURL, _ := url.JoinPath(c.MessagingURL, "messaging")
	params := url.Values{}
	params.Add("channel_id", channelID)
	endpoind, _ := url.Parse(msgPubURL)
	endpoind.RawQuery = params.Encode()

	req, err := http.NewRequest("POST", endpoind.String(), bytes.NewBuffer(msg))
	if err != nil {
		return err
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		decoder := json.NewDecoder(res.Body)
		var er ErrorsResult
		err := decoder.Decode(&er)
		if err == nil {
			return er
		} else {
			return fmt.Errorf("error result decode error: %w", err)
		}
	}

	return nil
}
