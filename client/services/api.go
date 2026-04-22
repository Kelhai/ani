package services

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Kelhai/ani/client/config"
	"github.com/google/uuid"
)

var (
	ErrPayloadMarshalFailed = errors.New("Failed to marshal payload")
)

var (
	SessionToken uuid.UUID

	cert []byte
	pool *x509.CertPool

	apiService ApiService

	httpClient *http.Client
)

type ApiService struct{}

func SetupApiService() {
	var err error
	cert, err = os.ReadFile("cert.pem")
	if err != nil {
		log.Fatal("Failed to read cert")
	}
	pool = x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(cert); !ok {
		log.Fatal("failed to parse cert")
	}
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}
	apiService = ApiService{}
}

func (_ ApiService) RawRequest(method, path string, payload any, headers map[string]string) (int, []byte, error) {
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

	for title, value := range headers {
		req.Header.Set(title, value)
	}

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

func (as ApiService) Request(method, path string, payload any) (int, []byte, error) {
	return as.RawRequest(method, path, payload, map[string]string{
		"Content-Type": "application/json",
		"Authorization": "Bearer " + SessionToken.String(),
	})
}

func (as ApiService) GET(path string, payload any) (int, []byte, error) {
	return as.Request("GET", path, payload)
}

func (as ApiService) HEAD(path string, payload any) (int, []byte, error) {
	return as.Request("HEAD", path, payload)
}

func (as ApiService) POST(path string, payload any) (int, []byte, error) {
	return as.Request("POST", path, payload)
}

func (as ApiService) PUT(path string, payload any) (int, []byte, error) {
	return as.Request("PUT", path, payload)
}

func (as ApiService) DELETE(path string, payload any) (int, []byte, error) {
	return as.Request("DELETE", path, payload)
}

func (as ApiService) CONNECT(path string, payload any) (int, []byte, error) {
	return as.Request("CONNECT", path, payload)
}

func (as ApiService) OPTIONS(path string, payload any) (int, []byte, error) {
	return as.Request("OPTIONS", path, payload)
}

func (as ApiService) TRACE(path string, payload any) (int, []byte, error) {
	return as.Request("TRACE", path, payload)
}

func (as ApiService) PATCH(path string, payload any) (int, []byte, error) {
	return as.Request("PATCH", path, payload)
}

func (as ApiService) QUERY(path string, payload any) (int, []byte, error) {
	return as.Request("QUERY", path, payload)
}

