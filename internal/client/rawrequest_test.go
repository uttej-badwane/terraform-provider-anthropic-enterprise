package client_test

import "net/http"

// newRawRequest issues a bare GET so header enforcement can be tested
// independently of the client's own header handling.
func newRawRequest(url, apiKey, beta string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	if beta != "" {
		req.Header.Set("Anthropic-Beta", beta)
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
}
