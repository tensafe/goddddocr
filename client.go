package goddddocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultClientTimeout = 10 * time.Second

type OCRClient struct {
	baseURL       string
	httpClient    *http.Client
	maxImageBytes int64
}

type OCRClientOption func(*OCRClient)

func WithHTTPClient(client *http.Client) OCRClientOption {
	return func(c *OCRClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithClientTimeout(timeout time.Duration) OCRClientOption {
	return func(c *OCRClient) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

func WithClientMaxImageBytes(n int64) OCRClientOption {
	return func(c *OCRClient) {
		if n > 0 {
			c.maxImageBytes = n
		}
	}
}

func NewOCRClient(baseURL string, options ...OCRClientOption) *OCRClient {
	c := &OCRClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: DefaultClientTimeout,
		},
		maxImageBytes: DefaultMaxImageBytes,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

type RemoteClassifyOptions struct {
	PNGFix *bool
}

type RemoteClassifyResult struct {
	Result           string  `json:"result"`
	ProcessingTimeMS float64 `json:"processing_time_ms"`
	RequestID        string  `json:"request_id,omitempty"`
}

type RemoteError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *RemoteError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("goddddocr request failed: status=%d message=%s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("goddddocr request failed: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
}

func (c *OCRClient) ClassifyBytes(ctx context.Context, image []byte, options *RemoteClassifyOptions) (*RemoteClassifyResult, error) {
	if c == nil {
		return nil, fmt.Errorf("nil OCR client")
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("base URL is empty")
	}
	if len(image) == 0 {
		return nil, fmt.Errorf("image is empty")
	}
	if c.maxImageBytes > 0 && int64(len(image)) > c.maxImageBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", c.maxImageBytes)
	}

	reqBody := ocrRequest{
		Image: base64.StdEncoding.EncodeToString(image),
	}
	if options != nil {
		reqBody.PNGFix = options.PNGFix
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ocr", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes()))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseRemoteError(resp, body)
	}

	var result RemoteClassifyResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode OCR response: %w", err)
	}
	return &result, nil
}

func (c *OCRClient) Ready(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("nil OCR client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseRemoteError(resp, body)
	}
	return nil
}

func (c *OCRClient) maxBodyBytes() int64 {
	if c.maxImageBytes <= 0 {
		return DefaultMaxBodyBytes
	}
	return c.maxImageBytes*4/3 + 4096
}

func parseRemoteError(resp *http.Response, body []byte) error {
	var payload errorResponse
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		return &RemoteError{
			StatusCode: resp.StatusCode,
			Code:       payload.Error.Code,
			Message:    payload.Error.Message,
			RequestID:  payload.RequestID,
		}
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return &RemoteError{
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}
