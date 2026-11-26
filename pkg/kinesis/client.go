package kinesis

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	awskinesis "github.com/aws/aws-sdk-go-v2/service/kinesis"
)

// KinesisEventUploadEvent is a single event entry that appends the MachineID with the EventUploadEvent details
type KinesisEvents struct {
	Items []interface{}
}

type KinesisClient interface {
	Send(machineID string, events KinesisEvents) (err error)
}

func GetClient(kinesisStreamName string, awsregion string) (KinesisClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(awsregion))
	if err != nil {
		return nil, err
	}

	return client{
		kinesisStream:  kinesisStreamName,
		kinesisService: awskinesis.NewFromConfig(cfg),
	}, nil
}

type client struct {
	kinesisStream  string
	kinesisService *awskinesis.Client
}
