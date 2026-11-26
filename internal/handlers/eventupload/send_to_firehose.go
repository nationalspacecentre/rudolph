package eventupload

import (
	"fmt"
	"log/slog"

	"github.com/airbnb/rudolph/pkg/firehose"
)

func sendToFirehose(firehoseClient firehose.FirehoseClient, machineID string, events []EventUploadEvent) error {
	var forwardedEvents = convertRequestEventsToUploadEvents(machineID, events)
	err := firehoseClient.Send(machineID, firehose.FirehoseEvents{Items: forwardedEvents})
	if err != nil {
		slog.Error("Event forwarding failed", "destination", "Firehose")
		return fmt.Errorf("failed to events to AWS Firehose: %w", err)
	}

	return nil
}
