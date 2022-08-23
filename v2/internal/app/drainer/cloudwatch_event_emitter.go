package drainer

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"strconv"

	"github.com/olebedev/emitter"                    // Event bus.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/mintel/elasticsearch-asg/v2/pkg/events" // AWS CloudWatch Events.
)

// CloudWatchEventEmitter consumes CloudWatch events from an SQS
// queue and emits them as github.com/olebedev/emitter events.
type CloudWatchEventEmitter struct {
	client sqs.Client
	queue  string
	events *emitter.Emitter

	// Metrics.
	Received prometheus.Counter
	Deleted  prometheus.Counter
}

// NewCloudWatchEventEmitter returns a new CloudWatchEventEmitter.
func NewCloudWatchEventEmitter(c sqs.Client, queueURL string, e *emitter.Emitter) *CloudWatchEventEmitter {
	return &CloudWatchEventEmitter{
		client: c,
		queue:  queueURL,
		events: e,
	}
}

// Run receives and emits CloudWatch events until the context is canceled
// or an error occurs.
func (e *CloudWatchEventEmitter) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			// Receive SQS messages.
			msgs, err := e.receive(ctx)
			if err != nil {
				return err
			}

			// Unmarshal and emit events.
			toWait := make(emitWaiter, 0, len(msgs))
			for _, m := range msgs {
				cwEvent := &events.CloudWatchEvent{}
				if err := json.Unmarshal([]byte(*m.Body), cwEvent); err != nil {
					zap.L().DPanic("error unmarshaling CloudWatch Event",
						zap.Error(err))
					continue
				}
				c := e.events.Emit(topicKey(cwEvent.Source, cwEvent.DetailType), cwEvent)
				toWait = append(toWait, c)
			}

			// Wait for events to be emitted.
			toWait.Wait()

			// Delete SQS messages.
			if err := e.delete(ctx, msgs); err != nil {
				return err
			}
		}
	}
}

// receive receives SQS messages.
func (e *CloudWatchEventEmitter) receive(ctx context.Context) ([]types.Message, error) {
	resp, err := e.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(e.queue),
		MaxNumberOfMessages: int32(10), // Max allowed by the AWS API.
		WaitTimeSeconds:     int32(20), // Max allowed by the AWS API.
	})
	//resp, err := req.Send(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting SQS messages")
	}
	if e.Received != nil {
		e.Received.Add(float64(len(resp.Messages)))
	}
	return resp.Messages, nil
}

// delete deletes SQS messages.
func (e *CloudWatchEventEmitter) delete(ctx context.Context, msgs []types.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	b := make([]types.DeleteMessageBatchRequestEntry, len(msgs))
	for i, m := range msgs {
		b[i] = types.DeleteMessageBatchRequestEntry{
			//QueueUrl: aws.String(e.queue),
			Id:            aws.String(strconv.Itoa(i)),
			ReceiptHandle: m.ReceiptHandle,
		}
	}
	_, err := e.client.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(e.queue),
		Entries:  b,
	})
	if err != nil {
		return errors.Wrap(err, "error deleting SQS messages")
	}
	if e.Deleted != nil {
		e.Deleted.Add(float64(len(msgs)))
	}
	return nil
}
