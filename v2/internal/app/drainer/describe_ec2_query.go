package drainer

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
	"strings"
)

// Ec2Instances represents the ec2 instances that are returned as result
// of an query
type DescribeEC2Action struct {
	client *ec2.Client

	logger *zap.Logger

	params *ec2.DescribeInstancesInput
}


type EC2Instance struct {

	ReservationId string

	InstanceId string

	PrivateIPAddress string

	InstanceName string
}

func NewDescribeEC2Action(action DescribeEC2Action) (map[string]EC2Instance, error) {

	result, err := GetInstances(context.TODO(), action.client, action.params)
	if err != nil {
		action.logger.Warn("Got an error retrieving information about your Amazon EC2 instances:",
			zap.Error(err))
	}
	if len(result.Reservations) == 0 {
		action.logger.Info("node with instance id: " + action.params.InstanceIds[0] + " is not part of this cluster")
		return make(map[string]EC2Instance), nil
	}

	ec2Instances := make(map[string]EC2Instance, len(action.params.InstanceIds))
	ips := make([]string, len(action.params.InstanceIds))
	rids := make([]string, len(action.params.InstanceIds))
	names := make([]string, len(action.params.InstanceIds))
	for i, r := range result.Reservations {
		action.logger.Info("Reservation ID: " + *r.ReservationId)
		rids[i] = *r.ReservationId
		action.logger.Info("Instance IDs:")
		for j, instance := range r.Instances {
			if strings.EqualFold(string(instance.State.Name), "running") {
				action.logger.Info("   " + *instance.InstanceId)
				if &instance.PrivateIpAddress != nil {
					action.logger.Info("   " + *instance.PrivateIpAddress)
					ips[j] = *instance.PrivateIpAddress
				}
				var nt string
				for _, t := range instance.Tags {
					if *t.Key == "Name" {
						nt = *t.Value
						break
					}
				}
				action.logger.Info(string("   " + instance.State.Name))
				action.logger.Info("    " + nt)
				names[j] = nt

				ec2Instances[*instance.InstanceId] = EC2Instance{
					ReservationId:    *r.ReservationId,
					InstanceId:       *instance.InstanceId,
					PrivateIPAddress: *instance.PrivateIpAddress,
					InstanceName:     nt,
				}
			} else {
				var nt string
				for _, t := range instance.Tags {
					if *t.Key == "Name" {
						nt = *t.Value
						break
					}
				}

				ec2Instances[*instance.InstanceId] = EC2Instance{
					ReservationId:    *r.ReservationId,
					InstanceId:       *instance.InstanceId,
					PrivateIPAddress: "",
					InstanceName:     nt,
				}
			}

		}

	}
	return ec2Instances, err
}

func getPrivateIps(client *ec2.Client, logger *zap.Logger, clusterName string, instanceIds []string) ([]string, error) {
	filter := types.Filter{}
	if clusterName != "" {
		filter = types.Filter{
			Name:   aws.String("tag:Name"),
			Values: []string{*aws.String(clusterName + "*")},
		}
	}

	params := &ec2.DescribeInstancesInput{
		DryRun: new(bool),
		//NextToken:   new(string),
		Filters:     []types.Filter{filter},
		InstanceIds: instanceIds,
		//MaxResults:  new(int32),
	}

	action := DescribeEC2Action{
		logger: logger,
		client: client,
		params: params,
	}

	ec2Instances, err := NewDescribeEC2Action(action)
	privateIps := make([]string, 0, len(ec2Instances))
	for _, value := range ec2Instances {
		privateIps = append(privateIps, value.PrivateIPAddress)
	}
	return privateIps, err
}

func getInstances(client *ec2.Client, logger *zap.Logger, clusterName string, instanceIds []string) (map[string]EC2Instance, error) {
	filter := types.Filter{}
	if clusterName != "" {
		filter = types.Filter{
			Name:   aws.String("tag:Name"),
			Values: []string{*aws.String(clusterName + "*")},
		}
	}

	params := &ec2.DescribeInstancesInput{
		DryRun: new(bool),
		Filters:     []types.Filter{filter},
		InstanceIds: instanceIds,
	}

	action := DescribeEC2Action{
		logger: logger,
		client: client,
		params: params,
	}

	return NewDescribeEC2Action(action)
}


// EC2DescribeInstancesAPI defines the interface for the DescribeInstances function.
// We use this interface to test the function using a mocked service.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// GetInstances retrieves information about your Amazon Elastic Compute Cloud (Amazon EC2) instances.
// Inputs:
//     c is the context of the method call, which includes the AWS Region.
//     api is the interface that defines the method call.
//     input defines the input arguments to the service call.
// Output:
//     If success, a DescribeInstancesOutput object containing the result of the service call and nil.
//     Otherwise, nil and an error from the call to DescribeInstances.
func GetInstances(c context.Context, client *ec2.Client, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return client.DescribeInstances(c, input)
}

