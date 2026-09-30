package telemetry

import (
	"net/http"
	"time"
)

func NewHTTPClient(provider string, timeout time.Duration, recorder Recorder) *http.Client {
	if recorder == nil {
		recorder = Noop()
	}
	return &http.Client{Timeout: timeout, Transport: roundTripper{provider: provider, recorder: recorder, next: http.DefaultTransport}}
}

type roundTripper struct {
	provider string
	recorder Recorder
	next     http.RoundTripper
}

func (t roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	started := time.Now().UTC()
	response, err := t.next.RoundTrip(request)
	finished := time.Now().UTC()
	status, httpStatus := "success", 0
	if err != nil {
		status = "error"
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		status = "error"
		httpStatus = response.StatusCode
	} else {
		httpStatus = response.StatusCode
	}
	operation := Operation(request.Context())
	if operation == "" {
		operation = "http_request"
	}
	errorType := ErrorType(err)
	if errorType == "" && status == "error" {
		errorType = "http_status"
	}
	t.recorder.Record(Event{AnalysisID: AnalysisID(request.Context()), Provider: t.provider, Operation: operation, Endpoint: Endpoint(request.URL.String()), Category: Category(request.Context()), StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds(), Status: status, ErrorType: errorType, HTTPStatus: httpStatus})
	return response, err
}
