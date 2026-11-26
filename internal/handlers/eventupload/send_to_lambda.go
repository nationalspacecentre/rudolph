package eventupload

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/airbnb/rudolph/pkg/lambda"
)

const RUDOLPH_DIRECT_SOURCE = "rudolph-direct"

func sendToLambda(
	ctx context.Context,
	lambdaClient lambda.LambdaClient,
	machineID string,
	events []EventUploadEvent,
) error {
	var forwardedEvents = convertRequestEventsToUploadEvents(machineID, events)
	err := lambdaClient.Send(
		ctx,
		machineID,
		lambda.LambdaEvents{
			Source: RUDOLPH_DIRECT_SOURCE,
			Items:  forwardedEvents,
		},
	)
	if err != nil {
		slog.Error("Event forwarding failed", "destination", "Lambda")
		return fmt.Errorf("failed to events to AWS Lambda: %w", err)
	}

	return nil
}
