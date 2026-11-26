package handlers

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/airbnb/rudolph/internal/handlers/authorizer"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/stretchr/testify/require"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

func TestSantaProxyRoutesAndSafeLogs(t *testing.T) {
	logs := captureLogs(t)
	t.Setenv("HANDLER", "NONE")
	t.Setenv("LAMBDA_NAME", "")
	t.Setenv("REGION", "eu-west-2")
	configDir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(configDir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(configDir, "credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	ctx := lambdacontext.NewContext(context.Background(), &lambdacontext.LambdaContext{AwsRequestID: "test-invocation"})

	const machineID = "fccdd011-d884-4bf8-b147-af577184178b"
	const eventBody = `{"events":[{"file_path":"/sensitive/executable","executing_user":"private-user","signing_chain":[{"cn":"private-certificate"}]}]}`
	for _, tc := range []struct {
		name     string
		resource string
		method   string
		body     string
		id       string
		status   int
	}{
		{"preflight rejects null", "/preflight/{machine_id}", "POST", "null", machineID, http.StatusBadRequest},
		{"eventupload rejects null", "/eventupload/{machine_id}", "POST", "null", machineID, http.StatusBadRequest},
		{"eventupload accepts events", "/eventupload/{machine_id}", "POST", eventBody, machineID, http.StatusOK},
		{"ruledownload rejects null", "/ruledownload/{machine_id}", "POST", "null", machineID, http.StatusBadRequest},
		{"postflight requires a machine ID", "/postflight/{machine_id}", "POST", "", "", http.StatusBadRequest},
		{"xsrf returns success", "/xsrf/{machine_id}", "POST", eventBody, machineID, http.StatusOK},
		{"unmatched route", "/unmatched-private-resource", "PRIVATE-METHOD", eventBody, machineID, http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := events.APIGatewayProxyRequest{
				Resource:   tc.resource,
				HTTPMethod: tc.method,
				Path:       "/private-path/" + tc.id,
				Headers:    map[string]string{"Content-Type": "application/json", "Authorization": "private-auth-token"},
				PathParameters: map[string]string{
					"machine_id": tc.id,
				},
				Body: tc.body,
			}
			req.RequestContext.Identity.ClientCert = &events.APIGatewayCustomAuthorizerRequestTypeRequestIdentityClientCert{ClientCertPem: "private-client-cert"}
			resp, err := ApiRouter(ctx, req)
			require.NoError(t, err)
			require.Equal(t, tc.status, resp.StatusCode)
			if tc.status == http.StatusOK {
				require.JSONEq(t, `{"status":"ok"}`, resp.Body)
			}
			_, err = authorizer.HandleAuthorizerRequest(req)
			require.NoError(t, err)
		})
	}

	output := logs.String()
	for _, secret := range []string{machineID, "private-path", "private-auth-token", "private-certificate",
		"private-client-cert", "private-user", "/sensitive/executable", "unmatched-private-resource", "PRIVATE-METHOD",
		"Authorization", "signing_chain", `"status":"ok"`,
	} {
		require.NotContains(t, output, secret)
	}
	require.Contains(t, output, `"route":"/eventupload/{machine_id}"`)
	require.Contains(t, output, `"request_id":"test-invocation"`)
	require.Contains(t, output, `"status":405`)
	require.Contains(t, output, `"decision":"allow"`)
	require.Contains(t, output, `"decision":"deny"`)
}

type failingHandler struct{}

func (failingHandler) Handles(events.APIGatewayProxyRequest) bool { return true }
func (failingHandler) Boot() error                                { return nil }
func (failingHandler) Handle(events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	return nil, errors.New("internal-error-with-private-data")
}

func TestRouterSanitizesLambdaErrors(t *testing.T) {
	logs := captureLogs(t)
	original := handlers
	handlers = []HandlerInterface{failingHandler{}}
	t.Cleanup(func() { handlers = original })

	_, err := ApiRouter(context.Background(), events.APIGatewayProxyRequest{})
	require.EqualError(t, err, "request handling failed")
	require.Contains(t, logs.String(), "Handler failed")
	require.NotContains(t, logs.String(), "internal-error-with-private-data")
}
