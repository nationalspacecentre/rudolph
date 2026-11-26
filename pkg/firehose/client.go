package firehose

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
)

type FirehoseClient interface {
	Send(machineID string, events FirehoseEvents) (err error)
}

type client struct {
	firehoseStreamName string
	firehoseService    putRecordBatchAPI
}

type putRecordBatchAPI interface {
	PutRecordBatch(context.Context, *firehose.PutRecordBatchInput, ...func(*firehose.Options)) (*firehose.PutRecordBatchOutput, error)
}

func GetClient(firehoseStreamName string, awsregion string) (FirehoseClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(awsregion))
	if err != nil {
		return nil, err
	}

	return client{
		firehoseStreamName: firehoseStreamName,
		firehoseService:    firehose.NewFromConfig(cfg),
	}, nil
}
