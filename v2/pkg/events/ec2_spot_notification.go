package events

import (
	"reflect"
)

func init() {

	MustRegisterDetailType(
		"aws.ec2",
		"EC2 Instance Rebalance Recommendation",
		reflect.TypeOf(EC2SpotNotification{}),
	)
}

// SpotInterruptionEventDetail is one possible value for the
// CloudWatchEvent.Detail field.
//
//   When Amazon EC2 is going to interrupt your Spot Instance,
//   it emits an event two minutes prior to the actual interruption.
//
// Example:
//
// {
//    "version": "0",
//    "id": "a6ade63d-f480-b014-642c-cfd2c0e18123",
//    "detail-type": "EC2 Instance Rebalance Recommendation",
//    "source": "aws.ec2",
//    "account": "807891339983",
//    "time": "2022-09-02T11:05:53Z",
//    "region": "eu-central-1",
//    "resources": ["arn:aws:ec2:eu-central-1:807891339983:instance/i-06428afec3a43f37c"],
//    "detail":
//    {
//        "instance-id": "i-06428afec3a43f37c"
//    }
//}
//
type EC2SpotNotification struct {
	// The ID of the EC2 spot instance that is about
	// to be interrupted.
	InstanceID string `json:"instance-id"`
}

