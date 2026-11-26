package postflight

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/airbnb/rudolph/pkg/clock"
	"github.com/airbnb/rudolph/pkg/dynamodb"
	"github.com/airbnb/rudolph/pkg/response"
	"github.com/aws/aws-lambda-go/events"
)

// PostflightHandler is the entry point for the /postflight API call
type PostPostflightHandler struct {
	booted           bool
	ruleDestroyer    staleRuleDestroyer
	syncStateUpdater syncStateUpdater
}

func (h *PostPostflightHandler) Boot() (err error) {
	if h.booted {
		return
	}

	dynamodbTableName := os.Getenv("DYNAMODB_NAME")
	awsRegion := os.Getenv("REGION")

	client := dynamodb.GetClient(dynamodbTableName, awsRegion)

	h.ruleDestroyer = concreteRuleDestroyer{
		queryer: client,
		deleter: client,
	}
	h.syncStateUpdater = concreteSyncStateUpdater{
		timeProvider: clock.ConcreteTimeProvider{},
		updater:      client,
	}

	h.booted = true
	return
}

func (h *PostPostflightHandler) Handles(request events.APIGatewayProxyRequest) bool {
	return request.Resource == "/postflight/{machine_id}" && request.HTTPMethod == "POST"
}

func (h *PostPostflightHandler) Handle(request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	machineID, _, errResponse, err := parseRequest(request)
	if errResponse != nil {
		return errResponse, nil
	}
	if err != nil {
		// Unreachable code; API Gateway should never encounter an error attempting to GetMachineID
		return errResponse, err
	}

	// FIXME (derek.wang) This postflight is not properly setting the FeedSyncCursor; it's just setting it to
	// like "10 minutes ago and leaving it like that. Pretty dumb, derek!"
	err = h.syncStateUpdater.updatePostflightDate(machineID)
	if err != nil {
		slog.Error("Failed to record postflight completion")
		return response.APIResponse(http.StatusInternalServerError, response.ErrInternalServerErrorResponse)
	}

	// Delete rules marked for deletion
	err = h.ruleDestroyer.destroyMachineRulesMarkedForDeletion(machineID)
	if err != nil {
		slog.Error("Failed to delete stale machine rules")
		return response.APIResponse(http.StatusInternalServerError, response.ErrInternalServerErrorResponse)
	}

	// Optional: Archive the sync state
	// _ = archiveSyncState(h.dynamodbClient, machineID)

	return response.APIResponse(http.StatusOK, map[string]string{"status": "ok"})
}
