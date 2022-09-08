package drainer

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"strconv"

	"github.com/olebedev/emitter"                    // Event bus.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/CompareGroup/elasticsearch-asg/v2/pkg/events" // AWS CloudWatch Events.
)

// CloudWatchEventEmitter consumes CloudWatch events from an SQS
// queue and emits them as github.com/olebedev/emitter events.
type CloudWatchEventEmitter struct {
	client sqs.Client
	queue  string
	events *emitter.Emitter

	ec2Client *ec2.Client
	clusterName string
	logger *zap.Logger

	// Metrics.
	Received prometheus.Counter
	Deleted  prometheus.Counter
}

// NewCloudWatchEventEmitter returns a new CloudWatchEventEmitter.
func NewCloudWatchEventEmitter(c sqs.Client, queueURL string, e *emitter.Emitter, ec2Client *ec2.Client, logger *zap.Logger, clusterName string) *CloudWatchEventEmitter {
	return &CloudWatchEventEmitter{
		client: c,
		queue:  queueURL,
		events: e,
		ec2Client: ec2Client,
		logger: logger,
		clusterName: clusterName,
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

			nodeIds := make([]string, len(msgs))
			nodeIdMessagesMap := make(map[string][]types.Message)
			nodeIdEventsMap := make(map[string][]events.CloudWatchEvent)
			var clusterMsgs []types.Message
			// Unmarshal and emit events.
			toWait := make(emitWaiter, 0, len(msgs))
			for _, m := range msgs {
				cwEvent := &events.CloudWatchEvent{}
				if err := json.Unmarshal([]byte(*m.Body), cwEvent); err != nil {
					zap.L().DPanic("error unmarshaling CloudWatch Event",
						zap.Error(err))
					continue
				}

				// create map with ids
				var nodeId string
				if cwEvent.Source == "aws.ec2" && cwEvent.DetailType == "EC2 Spot Instance Interruption Warning"  {
					nodeId = cwEvent.Detail.(*events.EC2SpotInterruption).InstanceID
				} else if cwEvent.Source == "aws.ec2" && cwEvent.DetailType == "EC2 Instance Rebalance Recommendation" {
					nodeId = cwEvent.Detail.(*events.EC2SpotNotification).InstanceID
				} else if cwEvent.Source == "aws.autoscaling" && cwEvent.DetailType == "EC2 Instance-terminate Lifecycle Action" {
					nodeId = cwEvent.Detail.(*events.AutoScalingLifecycleTerminateAction).EC2InstanceID
				} else {
					// add list of messages to be removed
					clusterMsgs = append(clusterMsgs, m)
				}

				if nodeId != "" {
					nodeIds = append(nodeIds, nodeId)
					nodeIdMessagesMap[nodeId] = append(nodeIdMessagesMap[nodeId], m)
					nodeIdEventsMap[nodeId] = append(nodeIdEventsMap[nodeId], *cwEvent)
				}
				nodeIds = removeEmptyStrings(nodeIds)

				var fCwEvents []events.CloudWatchEvent
				if len(nodeIds) > 0 {
					// filter based on cluster nodes
					instances, err := getInstances(e.ec2Client, e.logger, e.clusterName, nodeIds)
					// if instance from the msg does not exist anymore
					if err != nil {
						clusterMsgs = append(clusterMsgs, m)
					}
					// if instance from the msg exists and belongs to cluster
					for _, instance := range instances {
						instanceId := instance.InstanceId
						for _, fMsg := range nodeIdMessagesMap[instanceId] {
							clusterMsgs = append(clusterMsgs, fMsg)
						}
						for _, fCwEvent := range nodeIdEventsMap[instanceId] {
							fCwEvents = append(fCwEvents, fCwEvent)
						}
					}
				}

				for _, fCwEvent := range fCwEvents {
					c := e.events.Emit(topicKey(fCwEvent.Source, fCwEvent.DetailType), fCwEvent)
					toWait = append(toWait, c)
				}
			}

			// Wait for events to be emitted.
			toWait.Wait()


			//// Delete SQS messages.
			if err := e.delete(ctx, clusterMsgs); err != nil {
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

func removeEmptyStrings(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
