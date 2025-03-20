package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/gcp-filestore-csi-driver/pkg/common"
)

const (
	ProcessStartTimeMetric = "process_start_time_seconds"
)

func TestProcessStartTimeMetricExist(t *testing.T) {
	mm := NewMetricsManager()
	metricsFamilies, err := mm.GetRegistry().Gather()
	if err != nil {
		t.Fatalf("Error fetching metrics: %v", err)
	}

	// check 'process_start_time_seconds' metric exist
	for _, metricsFamily := range metricsFamilies {
		if metricsFamily.GetName() == ProcessStartTimeMetric {
			return
		}
	}

	t.Fatalf("Metrics does not contain %v. Scraped content: %v", ProcessStartTimeMetric, metricsFamilies)
}

func TestErrorCodeLabelValue(t *testing.T) {
	testCases := []struct {
		name          string
		operationErr  error
		wantErrorCode string
	}{
		{
			name:          "Not googleapi.Error",
			operationErr:  errors.New("I am not a googleapi.Error"),
			wantErrorCode: "Internal",
		},
		{
			name:          "User error",
			operationErr:  &googleapi.Error{Code: http.StatusBadRequest, Message: "User error with bad request"},
			wantErrorCode: "InvalidArgument",
		},
		{
			name:          "googleapi.Error but not a user error",
			operationErr:  &googleapi.Error{Code: http.StatusInternalServerError, Message: "Internal error"},
			wantErrorCode: "Internal",
		},
		{
			name:          "context canceled error",
			operationErr:  context.Canceled,
			wantErrorCode: "Canceled",
		},
		{
			name:          "context deadline exceeded error",
			operationErr:  context.DeadlineExceeded,
			wantErrorCode: "DeadlineExceeded",
		},
		{
			name:          "status error with Aborted error code",
			operationErr:  status.Error(codes.Aborted, "aborted error"),
			wantErrorCode: "Aborted",
		},
		{
			name:          "user multiattach error",
			operationErr:  fmt.Errorf("The disk resource 'projects/foo/disk/bar' is already being used by 'projects/foo/instances/1'"),
			wantErrorCode: "Internal",
		},
		{
			name:          "TemporaryError that wraps googleapi error",
			operationErr:  common.NewTemporaryError(codes.Unavailable, &googleapi.Error{Code: http.StatusBadRequest, Message: "User error with bad request"}),
			wantErrorCode: "InvalidArgument",
		},
		{
			name:          "TemporaryError that wraps fmt.Errorf, which wraps googleapi error",
			operationErr:  common.NewTemporaryError(codes.Aborted, fmt.Errorf("got error: %w", &googleapi.Error{Code: http.StatusBadRequest, Message: "User error with bad request"})),
			wantErrorCode: "InvalidArgument",
		},
		{
			name:          "TemporaryError that wraps status error",
			operationErr:  common.NewTemporaryError(codes.Aborted, status.Error(codes.InvalidArgument, "User error with bad request")),
			wantErrorCode: "InvalidArgument",
		},
		{
			name:          "TemporaryError that wraps multiattach error",
			operationErr:  common.NewTemporaryError(codes.Unavailable, fmt.Errorf("The disk resource 'projects/foo/disk/bar' is already being used by 'projects/foo/instances/1'")),
			wantErrorCode: "Internal",
		},
	}

	for _, tc := range testCases {
		t.Logf("Running test: %v", tc.name)
		errCode := errorCodeLabelValue(tc.operationErr)
		if diff := cmp.Diff(tc.wantErrorCode, errCode); diff != "" {
			t.Errorf("%s: -want err, +got err\n%s", tc.name, diff)
		}
	}
}
