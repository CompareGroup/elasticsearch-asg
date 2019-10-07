package es

import (
	"context"
	"time"

	elastic "github.com/olivere/elastic/v7"
)

// DialContextRetry returns a new Elasticsearch client that uses
// exponential backoff to retry in case of errors. More importantly, it
// uses retry/backoff for the initial connection to Elasticsearch,
// which the standard elastic.NewClient() func doesn't.
//
// If the max duration <= 0, a client without retry is returned.
// DialContextRetry won't retry on non-connection errors.
func DialContextRetry(ctx context.Context, init, max time.Duration, options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	if max <= 0 {
		return elastic.DialContext(ctx, options...)
	}
	backoff := elastic.NewExponentialBackoff(init, max)
	retrier := elastic.NewBackoffRetrier(backoff)
	options = append(options, elastic.SetRetrier(retrier))
	var err error
	for i := 0; ; i++ {
		wait, tryAgain, _ := retrier.Retry(ctx, i, nil, nil, err)
		start := time.Now()
		c, err := elastic.DialContext(
			ctx,
			append(options, elastic.SetHealthcheckTimeoutStartup(wait))...,
		)
		if err == nil {
			return c, nil
		}
		if !elastic.IsConnErr(err) {
			return nil, err
		}
		if !tryAgain {
			return nil, err
		}
		timeTaken := time.Now().Sub(start)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(timeTaken - wait):
		}
	}
}

// DialRetry returns a new Elasticsearch client that uses
// exponential backoff to retry in case of errors. More importantly, it
// uses retry/backoff for the initial connection to Elasticsearch,
// which the standard elastic.NewClient() func doesn't.
//
// If the max duration <= 0, a client without retry is returned.
// DialRetry won't retry on non-connection errors.
func DialRetry(init, max time.Duration, options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	return DialContextRetry(context.Background(), init, max, options...)
}
