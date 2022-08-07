package config

import (
	"time"
)

type Config struct {
	HTTPChecks map[string]HTTPCheck `mapstructure:"httpChecks"`
}

// HTTPCheck configures a check for the response from a given URL.
// The only required field is `URL`, which must be a valid URL.
type HTTPCheck struct {
	// URL is the URL to  be checked.
	URL string `mapstructure:"url"`
	// Method is the HTTP method to use for this check.
	// Method is optional and defaults to `GET` if undefined.
	Method string `mapstructure:"method,omitempty"`
	// Body is an optional request body to be posted to the target URL.
	Body string `mapstructure:"body,omitempty"`
	// ExpectedStatus is the expected response status code, defaults to `200`.
	ExpectedStatus int `mapstructure:"expectedStatus,omitempty"`
	// ExpectedBody is optional; if defined, makes the check fail if the response body does not match
	ExpectedBody string `mapstructure:"expectedBody,omitempty"`
	// Timeout is the timeout used for the HTTP request, defaults to "1s".
	Timeout time.Duration `mapstructure:"timeout,omitempty"`
	// Headers to set on the request
	Headers map[string]string `mapstructure:"headers,omitempty"`
	// Interval defines how often the check should be executed, defaults to 30 seconds.
	Interval time.Duration `mapstructure:"interval,omitempty"`
}
