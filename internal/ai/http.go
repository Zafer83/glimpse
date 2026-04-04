package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// doJSONRequest performs an HTTP request with a JSON body, reads the response,
// and unmarshals it into the provided target. Returns the status code and an
// error if the request fails or the server returns a non-2xx status.
func doJSONRequest(method, url string, headers map[string]string, body []byte, target any) (int, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	if target != nil {
		if err := json.Unmarshal(rawBody, target); err != nil {
			return resp.StatusCode, fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return resp.StatusCode, nil
}

var jsonHeader = map[string]string{"Content-Type": "application/json"}
