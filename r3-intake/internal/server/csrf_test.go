package server

import "net/http"

const testCsrfToken = "test-csrf-token"

// csrfCookieForTest returns an r3_csrf cookie carrying the shared test token.
func csrfCookieForTest() *http.Cookie {
	return &http.Cookie{Name: csrfCookieName, Value: testCsrfToken}
}

// addCSRFToRequest adds both the r3_csrf cookie and the matching X-CSRF-Token
// header so state-changing integration tests pass the CSRF middleware.
func addCSRFToRequest(req *http.Request) {
	req.AddCookie(csrfCookieForTest())
	req.Header.Set(csrfHeaderName, testCsrfToken)
}
