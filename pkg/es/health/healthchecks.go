// Package health implements Elasticsearch healthchecks (using https://github.com/heptiolabs/healthcheck)
// to check the liveness and readiness of an Elasticsearch node.
package health

import (
	"net/http"
	"sync"
	"time"

	elastic "github.com/olivere/elastic/v7"
)

// DefaultHTTPTimeout is the default timeout for sending HTTP requests
// to Elasticsearch.
var DefaultHTTPTimeout = 500 * time.Millisecond

// lazyClient instantiates an Elasticsearch client.
// The elastic.New[Simple]Client() func returns an error if it
// can't immediately connected to Elasticsearch. This struct
// allows the healthchecks to try creating a client until it succeeds.
type lazyClient struct {
	// The URL of Elasticsearch.
	URL string

	// Timeout making HTTP requests to Elasticsearch.
	// The cluster state endpoint tends to hang if the node hasn't joined a cluster.
	// If not set, DefaultHTTPTimeout will be used.
	Timeout time.Duration

	client *elastic.Client
	mu     sync.Mutex
}

// Client returns an elastic client.
func (lc *lazyClient) Client() (*elastic.Client, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.client == nil {
		timeout := lc.Timeout
		if timeout == 0 {
			timeout = DefaultHTTPTimeout
		}
		client, err := elastic.NewSimpleClient(
			elastic.SetURL(lc.URL),
			elastic.SetHttpClient(&http.Client{
				Timeout: timeout,
			}),
		)
		if err != nil {
			return nil, err
		}
		lc.client = client
		return client, nil
	}
	return lc.client, nil
}

// // NewHandler returns an http Handler configured to test the liveness and readiness
// // of an Elasticsearch node at URL.
// func NewHandler(ctx context.Context, URL string) (healthcheck.Handler, error) {
// 	lc := &lazyClient{URL: URL}
// 	health := healthcheck.NewHandler()

// 	for name, check := range liveChecks {
// 		health.AddLivenessCheck(name, healthcheck.Check(func() error {
// 			return check(ctx, lc)
// 		}))
// 	}

// 	for name, check := range readyChecks {
// 		health.AddReadinessCheck(name, healthcheck.Check(func() error {
// 			return check(ctx, lc)
// 		}))
// 	}

// 	return health, nil
// }

// // NewMetricsHandler returns an *http.ServeMux that responds to healthcheck and Prometheus metrics requests.
// // It also returns the healthcheck.Handler is case you want to add additional checks.
// func NewMetricsHandler(ctx context.Context, URL string) (*http.ServeMux, healthcheck.Handler, error) {
// 	lc := &lazyClient{URL: URL}
// 	registry := prometheus.NewRegistry()
// 	health := healthcheck.NewMetricsHandler(registry, "elasticsearch")

// 	for name, check := range liveChecks {
// 		health.AddLivenessCheck(name, healthcheck.Check(func() error {
// 			return check(ctx, lc)
// 		}))
// 	}

// 	for name, check := range readyChecks {
// 		health.AddReadinessCheck(name, healthcheck.Check(func() error {
// 			return check(ctx, lc)
// 		}))
// 	}

// 	mux := http.NewServeMux()
// 	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
// 	mux.HandleFunc("/live", health.LiveEndpoint)
// 	mux.HandleFunc("/ready", health.ReadyEndpoint)
// 	return mux, health, nil
// }
