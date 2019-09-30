package healthchecker

import (
	"context"

	"github.com/mintel/healthcheck"         // Healthchecks framework.
	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/pkg/errors"                 // Wrap errors with stacktrace.
)

// CheckLiveHEAD returns a liveness healthcheck that
// checks if a HEAD request to / returns 200.
func CheckLiveHEAD(c *elastic.Client) healthcheck.Check {
	return func() error {
		resp, err := c.PerformRequest(context.Background(), elastic.PerformRequestOptions{
			Method: "HEAD",
			Path:   "/",
		})
		if err != nil {
			return errors.Wrap(err, "error communicating with Elasticsearch")
		}

		if resp.StatusCode != 200 {
			return errors.New("HEAD request returned non-200 status code")
		}

		return nil
	}
}
