package drainer

import (
	"fmt"
	"sort"
	"strings"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"go.uber.org/zap"                       // Logging.

	"github.com/CompareGroup/elasticsearch-asg/v2/pkg/es" // Extensions to the Elasticsearch client.
)

// ClusterState represents the state of an Elasticsearch
// cluster.
type ClusterState struct {
	// Nodes in the cluster.
	Nodes []string

	// Count of shards per-node.
	Shards map[string]int

	// Shard allocation exclusions.
	Exclusions *es.ShardAllocationExcludeSettings
}

// NewClusterState returns a new ClusterState.
func NewClusterState(i *elastic.NodesInfoResponse, s es.CatShardsResponse, set *es.ClusterGetSettingsResponse) *ClusterState {
	nodes := make([]string, 0, len(i.Nodes))
	for _, n := range i.Nodes {
		nodes = append(nodes, n.IP)
	}
	sort.Strings(nodes)

	shards := make(map[string]int, len(nodes))
	for _, sr := range s {
		if sr.IP != nil {
			for _, ip := range parseShardNodes(*sr.IP) {
				shards[ip]++
			}
		}
	}

	return &ClusterState{
		Nodes:      nodes,
		Shards:     shards,
		Exclusions: es.NewShardAllocationExcludeSettings(set.Transient),
	}
}

// HasNode returns true if a node with the given node is in
// the Elasticsearch cluster.
func (s *ClusterState) HasNode(name string) bool {
	if !sort.StringsAreSorted(s.Nodes) {
		zap.L().Panic("node slices must be sorted")
	}
	i := sort.SearchStrings(s.Nodes, name)
	return i < len(s.Nodes) && s.Nodes[i] == name
}

// HasNode returns true if a node with the given node is in
// the Elasticsearch cluster.
func (s *ClusterState) HasNodeByIP(instanceIp string) bool {
	if !sort.StringsAreSorted(s.Nodes) {
		zap.L().Panic("node slices must be sorted")
	}
	i := sort.SearchStrings(s.Nodes, instanceIp)
	return i < len(s.Nodes) && s.Nodes[i] == instanceIp
}

// DiffNodes returns the difference between the nodes of two cluster states.
func (s *ClusterState) DiffNodes(o *ClusterState) (add, remove []string) {
	if s == nil && o == nil {
		return nil, nil
	}
	if s == nil {
		add = append(add, o.Nodes...)
		return
	}
	if o == nil {
		remove = append(remove, s.Nodes...)
		return
	}
	if !(sort.StringsAreSorted(s.Nodes) && sort.StringsAreSorted(o.Nodes)) {
		zap.L().Panic("node slices must be sorted")
	}
	i, j := 0, 0
	for i < len(s.Nodes) && j < len(o.Nodes) {
		sn, on := s.Nodes[i], o.Nodes[j]
		if sn < on {
			remove = append(remove, sn)
			i++
		} else if sn > on {
			add = append(add, on)
			j++
		} else {
			i++
			j++
		}
	}
	if i < len(s.Nodes) {
		remove = append(remove, s.Nodes[i:]...)
	}
	if j < len(o.Nodes) {
		add = append(add, o.Nodes[j:]...)
	}
	return
}

// DiffNodes returns the difference between the shards of two cluster states.
func (s *ClusterState) DiffShards(o *ClusterState) map[string]int {
	if s == nil && o == nil {
		return nil
	}
	out := make(map[string]int)
	if s == nil {
		for n, c := range o.Shards {
			out[n] = c
		}
		return out
	}
	if o == nil {
		for n, c := range s.Shards {
			out[n] = -c
		}
		return out
	}
	for n, c := range s.Shards {
		if oc, ok := o.Shards[n]; ok {
			out[n] = oc - c
		} else {
			out[n] = -c
		}
	}
	for n, c := range o.Shards {
		if _, seen := s.Shards[n]; !seen {
			out[n] = c
		}
	}
	return out
}

// parseShardNodes parses the node name from the /_cat/shards endpoint response
//
// This could be one of:
// - An empty string for an unassigned shard.
// - A node name for an normal shard.
// - Multiple node names if the shard is being relocated.
func parseShardNodes(node string) []string {
	if node == "" {
		return nil
	}
	parts := strings.Fields(node)
	switch len(parts) {
	case 1: // Example: "i-0968d7621b79cd73d"
		return parts
	case 2,3,4,5:
		fmt.Printf("parts are", parts)
	case 6: // Example: "172.24.32.153 172-24-32-153-data-front-cg-p-prod -> 172.24.32.33 UNq6sOGNTxqyPEHJjj5haQ 172-24-32-33-data-front-cg-p-prod"
		return []string{parts[0], parts[3]}
	}
	//case 5: // Example: "i-0968d7621b79cd73d -> 10.2.4.58 kNe49LLvSqGXBn2s8Ffgyw i-0a2ed08df0e5cfff6"
	//	return []string{parts[0], parts[4]}
	//}
	zap.L().Panic("couldn't parse /_cat/shards response node name: " + node)
	return nil
}
