package firehose

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsfirehose "github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
	"github.com/stretchr/testify/require"
)

type putRecordBatchFunc func(*awsfirehose.PutRecordBatchInput) (*awsfirehose.PutRecordBatchOutput, error)

func (f putRecordBatchFunc) PutRecordBatch(_ context.Context, input *awsfirehose.PutRecordBatchInput, _ ...func(*awsfirehose.Options)) (*awsfirehose.PutRecordBatchOutput, error) {
	return f(input)
}

func TestSendRetriesOnlyFailedRecords(t *testing.T) {
	calls := 0
	c := client{firehoseStreamName: "test-stream", firehoseService: putRecordBatchFunc(func(input *awsfirehose.PutRecordBatchInput) (*awsfirehose.PutRecordBatchOutput, error) {
		calls++
		require.Equal(t, "test-stream", aws.ToString(input.DeliveryStreamName))
		require.Equal(t, "\"retry\"\n", string(input.Records[len(input.Records)-1].Data))
		if calls == 1 {
			require.Len(t, input.Records, 2)
			require.Equal(t, "\"accepted\"\n", string(input.Records[0].Data))
			return &awsfirehose.PutRecordBatchOutput{
				FailedPutCount: aws.Int32(1),
				RequestResponses: []types.PutRecordBatchResponseEntry{
					{RecordId: aws.String("success")},
					{ErrorCode: aws.String("ServiceUnavailableException")},
				},
			}, nil
		}
		require.Len(t, input.Records, 1)
		return &awsfirehose.PutRecordBatchOutput{FailedPutCount: aws.Int32(0)}, nil
	})}

	require.NoError(t, c.Send("machine-id", FirehoseEvents{Items: []interface{}{"accepted", "retry"}}))
	require.Equal(t, 2, calls)
}

func TestSendExhaustsRetries(t *testing.T) {
	calls := 0
	c := client{firehoseService: putRecordBatchFunc(func(input *awsfirehose.PutRecordBatchInput) (*awsfirehose.PutRecordBatchOutput, error) {
		calls++
		return &awsfirehose.PutRecordBatchOutput{
			FailedPutCount: aws.Int32(1),
			RequestResponses: []types.PutRecordBatchResponseEntry{
				{ErrorCode: aws.String("InternalFailure")},
			},
		}, nil
	})}
	require.EqualError(t, c.Send("machine-id", FirehoseEvents{Items: []interface{}{"retry"}}),
		"Firehose batch still has failed records after retries")
	require.Equal(t, maxAttempts, calls)
}
