package throttler

import (
	"context"
	"sync"

	elastic "github.com/olivere/elastic/v7"
	"golang.org/x/sync/errgroup"

	"github.com/mintel/elasticsearch-asg/pkg/es"
)

// ClusterStateGetter queries an Elasticsearch cluster to return
// status information about the cluster that is useful when deciding
// whether to allow scaling up or down of the Elasticsearch cluster.
type ClusterStateGetter struct {
	client    *elastic.Client
	lastState *ClusterState
	mu        sync.Mutex
}

// NewClusterStateGetter returns a new ClusterStateGetter.
func NewClusterStateGetter(client *elastic.Client) *ClusterStateGetter {
	return &ClusterStateGetter{
		client: client,
	}
}

// Get returns a ClusterState. If the previous call to Get returned
// a ClusterState whose Status was "red" or had relocating shards, this
// call will block until the cluster status is "yellow" or "green" and
// there are no relocating shards.
//
// Only one call to Get can proceed at a time. Concurrent calls will block.
func (sg *ClusterStateGetter) Get() (*ClusterState, error) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	cs := new(ClusterState)
	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		hs := sg.client.ClusterHealth()

		if sg.lastState != nil {
			if sg.lastState.Status == "red" {
				hs = hs.WaitForYellowStatus()
			}
			if sg.lastState.RelocatingShards {
				hs = hs.WaitForNoRelocatingShards(true)
			}
		}

		resp, err := hs.Do(ctx)
		if err != nil {
			return err
		}
		cs.Status = resp.Status
		cs.RelocatingShards = resp.RelocatingShards > 0
		return nil
	})

	g.Go(func() error {
		rs := es.NewIndicesRecoveryService(sg.client).
			ActiveOnly(true).
			Detailed(false)
		resp, err := rs.Do(ctx)
		if err != nil {
			return err
		}
		for _, idx := range resp {
			for _, s := range idx.Shards {
				if s.Type == "store" {
					cs.RecoveringFromStore = true
					return nil
				}
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sg.lastState = cs
	return cs, nil
}
