package drainer

import (
	"context"
	goerrors "errors"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto" // Cache.
	"go.uber.org/zap"                // Logging.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var (
	// ErrLifecycleActionTimeout is returned by PostponeLifecycleHookAction
	// when the lifecycle action times out (or isn't found in the first place).
	ErrLifecycleActionTimeout = goerrors.New("lifecycle action timed out")

	// ErrTestLifecycleAction is return by NewLifecycleAction when
	// the passed CloudWatchEvent doesn't represent a valid LifecycleAction.
	ErrInvalidLifecycleAction = goerrors.New("invalid lifecycle action")
)

// LifecycleActionPostponer prevents LifecycleActions from timing out.
// See the Postpone method for more details.
type LifecycleActionPostponer struct {
	client             autoscaling.Client
	lifecycleHookCache *ristretto.Cache
}

// NewLifecycleActionPostponer returns a new LifecycleActionPostponer.
func NewLifecycleActionPostponer(client autoscaling.Client) *LifecycleActionPostponer {
	// TODO: move this out of a global variable.
	lifecycleHookCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * 10,
		MaxCost:     10,
		BufferItems: 8,
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &LifecycleActionPostponer{
		client:             client,
		lifecycleHookCache: lifecycleHookCache,
	}
}

// Postpone postpones the timeout of a AWS AutoScaling Group
// Lifecycle Hook action until the context is canceled, an error occurs, or the
// Lifecycle Hook's global timeout is reached.
//
// If the action expires (or can't be found; there's no way to distinguish
// in the AWS API) then ErrLifecycleActionTimeout will be returned.
//
// See also: https://docs.aws.amazon.com/autoscaling/ec2/userguide/lifecycle-hooks.html#lifecycle-hooks-overview
func (lap *LifecycleActionPostponer) Postpone(ctx context.Context, c autoscaling.Client, a *LifecycleAction) error {
	// Get Lifecycle Hook description because we need to know
	// what the timeout for each action it.
	hook, err := lap.describeLifecycleHook(ctx, a.AutoScalingGroupName, a.LifecycleHookName)
	if err != nil {
		return err
	}

	timeoutD := time.Duration(*hook.HeartbeatTimeout) * time.Second
	globalTimeoutD := time.Duration(*hook.GlobalTimeout) * time.Second
	timeout := a.Start.Add(timeoutD)
	globalTimeout := time.NewTimer(globalTimeoutD)
	defer globalTimeout.Stop()
	halfWayToTimeout := time.NewTimer(timeout.Sub(time.Now()) / 2)
	defer halfWayToTimeout.Stop()

	heartbeatInput := &autoscaling.RecordLifecycleActionHeartbeatInput{
		AutoScalingGroupName: aws.String(a.AutoScalingGroupName),
		LifecycleHookName:    aws.String(a.LifecycleHookName),
	}
	if a.InstanceID != "" {
		heartbeatInput.InstanceId = aws.String(a.InstanceID)
	}
	if a.Token != "" {
		heartbeatInput.LifecycleActionToken = aws.String(a.Token)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-halfWayToTimeout.C:
			_, err := c.RecordLifecycleActionHeartbeat(ctx, heartbeatInput)
			if aerr, ok := err.(awserr.Error); ok {
				code := aerr.Code()
				msg := aerr.Message()
				if code == "ValidationError" && strings.HasPrefix(msg, "No active Lifecycle Action found with token") {
					return ErrLifecycleActionTimeout
				}
				return err
			} else if err != nil {
				return err
			}
			zap.L().Debug("recorded lifecycle action heartbeat",
				zap.String("autoscaling_group", a.AutoScalingGroupName),
				zap.String("lifecycle_hook", a.LifecycleHookName),
				zap.String("instance", a.InstanceID))
			timeout = timeout.Add(timeoutD)
			halfWayToTimeout.Reset(timeout.Sub(time.Now()) / 2)

		case <-globalTimeout.C:
			return ErrLifecycleActionTimeout
		}
	}
}

// describeLifecycleHook fetches a description of an AWS AutoScaling Group
// Lifecycle Hook.
func (lap *LifecycleActionPostponer) describeLifecycleHook(ctx context.Context, groupName, hookName string) (types.LifecycleHook, error) {
	var hook types.LifecycleHook

	cacheKey := groupName + ":" + hookName
	entry, ok := lap.lifecycleHookCache.Get(cacheKey)
	if ok {
		hook = entry.(types.LifecycleHook)
		zap.L().Debug("got lifecycle hook from cache",
			zap.String("autoscaling_group", *hook.AutoScalingGroupName),
			zap.String("lifecycle_hook", *hook.LifecycleHookName))

	} else {
		params := autoscaling.DescribeLifecycleHooksInput{
			AutoScalingGroupName: aws.String(groupName),
			LifecycleHookNames:   []string{hookName},
		}

		resp, err := lap.client.DescribeLifecycleHooks(
			ctx,
			&params,
			)
		//.DescribeLifecycleHooksRequest(&autoscaling.DescribeLifecycleHooksInput{
		//		AutoScalingGroupName: aws.String(groupName),
		//		LifecycleHookNames:   []string{hookName},
		//	})
		//resp, err := req.Send(ctx)
		if err != nil {
			return hook, err
		}
		if n := len(resp.LifecycleHooks); n != 1 {
			zap.L().Panic("got wrong number of lifecycle hooks",
				zap.Int("count", n))
		}
		hook = resp.LifecycleHooks[0]
		zap.L().Debug("described lifecycle hook",
			zap.String("autoscaling_group", *hook.AutoScalingGroupName),
			zap.String("lifecycle_hook", *hook.LifecycleHookName))
		lap.lifecycleHookCache.Set(cacheKey, hook, 1)

	}
	return hook, nil

}
