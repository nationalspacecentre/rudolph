package firehose

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsfirehose "github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

const (
	maxAttempts = 3
	maxRecords  = 500
)

type FirehoseEvents struct {
	Items []interface{}
}

// Send uploads newline-delimited JSON records, retrying only failed records
// when Firehose accepts part of a batch.
func (c client) Send(_ string, events FirehoseEvents) error {
	for batch := range slices.Chunk(events.Items, maxRecords) {
		records := make([]types.Record, 0, len(batch))
		for _, event := range batch {
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			records = append(records, types.Record{Data: append(data, '\n')})
		}

		if err := c.sendBatch(records); err != nil {
			return err
		}
	}
	return nil
}

func (c client) sendBatch(records []types.Record) error {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := c.firehoseService.PutRecordBatch(context.Background(), &awsfirehose.PutRecordBatchInput{
			DeliveryStreamName: aws.String(c.firehoseStreamName),
			Records:            records,
		})
		if err != nil {
			return err
		}
		failed := int(aws.ToInt32(result.FailedPutCount))
		if failed == 0 {
			return nil
		}
		if len(result.RequestResponses) != len(records) || failed < 0 || failed > len(records) {
			return errors.New("invalid Firehose batch response")
		}

		pending := make([]types.Record, 0, failed)
		for i, record := range result.RequestResponses {
			if record.ErrorCode != nil || record.ErrorMessage != nil {
				pending = append(pending, records[i])
			}
		}
		if len(pending) != failed {
			return errors.New("invalid Firehose failed record count")
		}

		if attempt < maxAttempts {
			slog.Warn("Retrying failed Firehose records", "failed_records", failed, "attempt", attempt+1)
		}
		records = pending
	}
	return errors.New("Firehose batch still has failed records after retries")
}
