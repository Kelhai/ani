package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Kelhai/ani/client/config"
	"github.com/google/uuid"
)

var (
	ErrPayloadMarshalFailed = errors.New("Failed to marshal payload")
)

var SessionToken uuid.UUID

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

// figure out authn

func Request(method, path string, payload any) (int, []byte, error) {
	var (
		b []byte
		err error
	)

	switch v := payload.(type) {
		case nil:
		case []byte:
			b = v
		case string:
			b = []byte(v)
		default:
			b, err = json.Marshal(v)
			if err != nil {
				return -1, nil, fmt.Errorf("Failed to marshal payload: %w: %w", ErrPayloadMarshalFailed, err)
			}
	}

	req, err := http.NewRequest(method, config.BaseUrl + path, bytes.NewReader(b))
	if err != nil {
		return -1, nil, fmt.Errorf("Failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer " + SessionToken.String())

	resp, err := httpClient.Do(req)
	if err != nil {
		return -2, nil, fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -3, nil, fmt.Errorf("Failed to read body: %w", err)
	}

	return resp.StatusCode, body, nil
}

