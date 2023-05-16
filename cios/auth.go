package cios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type DeviceAuthResult struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type DeviceAuthErrorResult struct {
	Msg         string `json:"error"`
	Description string `json:"error_description"`
}

func (e DeviceAuthErrorResult) Error() string {
	return fmt.Sprintf("error: %s, description: %s", e.Msg, e.Description)
}

func (c *CIOSDeviceClient) FetchAccessToken() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": c.ClientID,
		"sub": c.ClientID,
		"aud": c.AuthURL,
		"iat": now.Unix(),
		"exp": now.Add(3 * time.Minute).Unix(),
		"jti": uuid.New().String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	assertion, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("JWT signed error: %w", err)
	}

	ctx := context.Background()

	form := url.Values{}
	form.Add("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Add("assertion", assertion)
	form.Add("client_id", c.ClientID)
	form.Add("scope", strings.Join(c.Scope, " "))
	payload := strings.NewReader(form.Encode())

	endpoind, _ := url.JoinPath(c.AuthURL, "/connect/token")
	req, err := http.NewRequestWithContext(ctx, "POST", endpoind, payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.HttpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect token request error: %w", err)
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var r DeviceAuthResult
	if res.StatusCode != 200 {
		var er DeviceAuthErrorResult
		err = decoder.Decode(&er)
		if err == nil {
			return "", er
		}
	} else {
		err = decoder.Decode(&r)
	}
	if err != nil {
		return "", err
	}

	return r.AccessToken, nil
}

func (c *CIOSDeviceClient) UpdateAccessToken() error {
	accessToken, err := c.FetchAccessToken()
	if err != nil {
		return fmt.Errorf("access token update error: %w", err)
	}
	c.accessToken = accessToken
	return nil
}

func (c CIOSDeviceClient) shouldTokenUpdate() bool {
	if c.accessToken == "" {
		return true
	}
	token, err := jwt.Parse(c.accessToken, nil)
	if err != nil {
		return true
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		ExpirationUnixTime, _ := claims.GetExpirationTime()
		return ExpirationUnixTime.Unix()-time.Now().Unix() < 30
	} else {
		return true
	}
}
