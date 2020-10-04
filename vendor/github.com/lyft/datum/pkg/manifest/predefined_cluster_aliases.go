package manifest

var preDefinedClusterAliases = map[string]ClusterAlias{
	"core": {
		Name: "core",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role": {"core"},
		},
	},
	"core-prod": {
		Name: "core-prod",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role":        {"core"},
			"cluster.lyft.net/environment": {"production"},
		},
	},
	"core-staging": {
		Name: "core-staging",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role":        {"core"},
			"cluster.lyft.net/environment": {"staging"},
		},
	},
	"omnicron": {
		Name: "omnicron",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role": {"omnicron"},
		},
	},
	"core-staging-1": {
		Name: "core-staging-1",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-staging-1"},
		},
	},
	"core-staging-2": {
		Name: "core-staging-2",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-staging-2"},
		},
	},
	"core-staging-4": {
		Name: "core-staging-4",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-staging-4"},
		},
	},
	"omnicron-staging": {
		Name: "omnicron-staging",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role":         {"omnicron"},
			"cluster.lyft.net/cluster_name": {"omnicron-staging"},
		},
	},
	"omnicron-prod": {
		Name: "omnicron-prod",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role":         {"omnicron"},
			"cluster.lyft.net/cluster_name": {"omnicron-prod"},
		},
	},
	"core-prod-0": {
		Name: "core-prod-0",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-prod-0"},
		},
	},
	"core-prod-1": {
		Name: "core-prod-1",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-prod-1"},
		},
	},
	"core-prod-2": {
		Name: "core-prod-2",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-prod-2"},
		},
	},
	"core-prod-3": {
		Name: "core-prod-3",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-prod-3"},
		},
	},
	"core-prod-4": {
		Name: "core-prod-4",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/cluster_name": {"core-prod-4"},
		},
	},
	"deploys": {
		Name: "deploys",
		ClusterLabels: map[string][]string{
			"cluster.lyft.net/role": {"deploys"},
		},
		ClusterTolerations: map[string][]string{
			"cluster.lyft.net/restricted": {"infra"},
		},
	},
}
