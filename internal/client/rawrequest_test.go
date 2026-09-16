package client_test

import "net/http"

// newRawRequest issues a bare GET so header enforcement can be tested
// independently of the client's own header handling.
// rawResponse is the part of a response a test asserts on. The body is
// already closed by the time it is returned.
type rawResponse struct {
	StatusCode int
	Header     http.Header
}

func newRawRequest(url, apiKey, beta string) (rawResponse, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	if beta != "" {
		req.Header.Set("Anthropic-Beta", beta)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return rawResponse{}, err
	}
	defer resp.Body.Close()
	return rawResponse{StatusCode: resp.StatusCode, Header: resp.Header}, nil
}
