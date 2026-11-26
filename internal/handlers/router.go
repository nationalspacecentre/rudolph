package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/airbnb/rudolph/internal/handlers/eventupload"
	"github.com/airbnb/rudolph/internal/handlers/health"
	"github.com/airbnb/rudolph/internal/handlers/postflight"
	"github.com/airbnb/rudolph/internal/handlers/preflight"
	"github.com/airbnb/rudolph/internal/handlers/ruledownload"
	"github.com/airbnb/rudolph/internal/handlers/xsrf"
	"github.com/airbnb/rudolph/pkg/response"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

var (
	handlers []HandlerInterface
)

func init() {
	handlers = []HandlerInterface{
		&health.GetHealthHandler{},
		&eventupload.PostEventuploadHandler{},
		&preflight.PostPreflightHandler{},
		&ruledownload.PostRuledownloadHandler{},
		&postflight.PostPostflightHandler{},
		&xsrf.PostXSRFHandler{},
	}
}

func ApiRouter(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	logger := slog.Default()
	if invocation, ok := lambdacontext.FromContext(ctx); ok {
		logger = logger.With("request_id", invocation.AwsRequestID)
	}

	for _, h := range handlers {
		if !h.Handles(request) {
			continue
		}

		// Handles matches a fixed resource template and method before either is logged.
		logger = logger.With("route", request.Resource, "method", request.HTTPMethod)
		if err := h.Boot(); err != nil {
			logger.Error("Handler initialization failed", "status", http.StatusInternalServerError)
			return response.APIResponse(http.StatusInternalServerError, nil)
		}

		resp, err := h.Handle(request)
		if err != nil || resp == nil {
			logger.Error("Handler failed")
			// Lambda logs returned errors, so do not propagate an internal error message.
			return nil, errors.New("request handling failed")
		}
		level := slog.LevelInfo
		if resp.StatusCode >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		logger.Log(ctx, level, "Request completed", "status", resp.StatusCode)
		return resp, nil
	}

	logger.Warn("No matching route", "status", http.StatusMethodNotAllowed)
	return response.APIResponse(http.StatusMethodNotAllowed, nil)
}

type HandlerInterface interface {
	Handles(request events.APIGatewayProxyRequest) bool
	Boot() error
	Handle(request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error)
}
