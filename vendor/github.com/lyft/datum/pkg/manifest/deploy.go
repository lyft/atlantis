package manifest

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var validLabelKey = regexp.MustCompile("^[a-zA-Z0-9.-]+[/]?[a-zA-Z0-9_-]+$")
var validLabelValue = regexp.MustCompile("^[a-zA-Z0-9_-]+$")
var validAzRegex = regexp.MustCompile("([a-z]+)-((?:us|eu)-(?:west|east|central)-[1-9][a-f])")
var deployStep = regexp.MustCompile(`(?P<env>\w+)-?(?P<overlay>.*)?`)

const (
	stageProduction        string = "production"
	canaryStage            string = "canary"
	stageStaging           string = "staging"
	development            string = "development"
	defaultBakeTimeMinutes int    = 10
	defaultMaxCommits      int    = 20
)

var validEnvs = map[string]bool{development: true, stageProduction: true, stageStaging: true}

const hhmmLayout = "15:04"

type protoKubernetesDeploymentStage struct {
	// Old camelCased name so we can normalize to the new one
	ClusterLabelsOld      map[string]interface{} `mapstructure:"clusterLabels"`
	ClusterLabels         map[string]interface{} `mapstructure:"cluster_labels"`
	ClusterTolerations    map[string]interface{} `mapstructure:"cluster_tolerations"`
	Enabled               *bool
	Path                  string
	ACL                   map[string][]string
	Mode                  string
	DisabledInterpolation bool              `mapstructure:"disable_interpolation"`
	InterpolationVars     map[string]string `mapstructure:"interpolation_vars"`
}

type protoDeploymentStage struct {
	Name             string
	Environment      string
	Legacy           bool
	V3               *V3DeploymentStage
	Kubernetes       protoKubernetesDeploymentStage
	Automatic        bool
	BakeTimeMinutes  *int `mapstructure:"bake_time_minutes"`
	Links            []DeploymentLink
	AvailabilityZone *string      `mapstructure:"availability_zone"`
	WatchRemoteFiles []RemoteFile `mapstructure:"watch_remote_files"`
	Targets          []interface{}
	isAZSpecific     bool
	Role             *string
	Schedule         *string
}

type protoDeployTarget struct {
	Facet          string
	FacetGroup     string `mapstructure:"facet_group"`
	Locations      []DeployLocation
	ClusterAliases []string `mapstructure:"clusters"`
}

func populateDeployTarget(target interface{}, facets Facets, facetGroups FacetGroups, clusterAliases ClusterAliases) (map[string]DeployTarget, error) {
	var proto protoDeployTarget
	dts := make(map[string]DeployTarget)

	decoder, err := newDecoder(&proto)
	if err != nil {
		return nil, errors.Wrap(err, "can't build decoder for populate target")
	}
	err = decoder.Decode(target)
	if err != nil {
		return nil, errors.Wrap(err, "can't decode target")
	}

	if len(proto.ClusterAliases) != 0 {
		for _, clusterName := range proto.ClusterAliases {
			cluster, ok := clusterAliases.byName[clusterName]
			if ok {
				facet, isFacet := facets.byName[proto.Facet]
				facetGroup, isGroup := facetGroups.byName[proto.FacetGroup]
				if !isFacet && !isGroup {
					return nil, fmt.Errorf("can't locate facet or facet group named %v", proto.Facet)
				}

				if isFacet {
					facet = facet.Inherit(cluster)
					dts[fmt.Sprintf("%v-%v", facet.Facet, clusterName)] = DeployTarget{Facet: facet, Locations: proto.Locations, ClusterAlias: cluster}
				}

				if isGroup {
					for _, facet := range facetGroup.Members {
						facet = facet.Inherit(cluster)
						dts[fmt.Sprintf("%v-%v", facet.Facet, clusterName)] = DeployTarget{Facet: facet, Locations: proto.Locations, ClusterAlias: cluster}
					}
				}
			} else {
				return nil, fmt.Errorf("can't locate cluster_alias '%v'", clusterName)
			}
		}
	} else {
		facet, isFacet := facets.byName[proto.Facet]
		facetGroup, isGroup := facetGroups.byName[proto.FacetGroup]
		if !isFacet && !isGroup {
			return nil, fmt.Errorf("can't locate facet or facet group named %v", proto.Facet)
		}

		if isFacet {
			dts[facet.Facet] = DeployTarget{Facet: facet, Locations: proto.Locations}
		}

		if isGroup {
			for _, facet := range facetGroup.Members {
				dts[facet.Facet] = DeployTarget{Facet: facet, Locations: proto.Locations}
			}
		}
	}

	return dts, nil
}

func validateClusterTolerations(tolerations map[string][]string) error {
	for k, v := range tolerations {
		if !validLabelKey.MatchString(k) {
			return fmt.Errorf("invalid key %v", k)
		}
		for _, entry := range v {
			if entry == "*" {
				continue
			}
			if !validLabelValue.MatchString(entry) {
				return fmt.Errorf("invalid value %v in key %v", entry, k)
			}
		}
	}
	return nil
}

func populateKubernetesStage(inp protoKubernetesDeploymentStage) (KubernetesDeploymentStage, error) {
	var output KubernetesDeploymentStage

	// TODO(@theatrus) This needs a copy-like constructor with generation
	if inp.Enabled != nil {
		output.Enabled = *inp.Enabled
	}
	output.Path = inp.Path
	output.ACL = inp.ACL
	output.InterpolationVars = inp.InterpolationVars
	output.Mode = inp.Mode
	output.DisableInterpolation = inp.DisabledInterpolation
	output.ClusterLabels = make(map[string][]string)
	output.ClusterTolerations = make(map[string][]string)

	merge(inp.ClusterLabels, output.ClusterLabels)
	merge(inp.ClusterLabelsOld, output.ClusterLabels)
	merge(inp.ClusterTolerations, output.ClusterTolerations)

	return output, validateClusterTolerations(output.ClusterTolerations)
}

// populateDeploySettings populates a DeploySettings struct.
// WindowStart and WindowEnd must either all be present and valid or all absent.
func populateDeploySettings(root interface{}, deploySettings *DeploySettings) error {
	rawMap, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}

	rawStart, ok := rawMap["window_start"].(string)
	if !ok {
		return fmt.Errorf("unable to parse window_start string")
	}
	rawEnd, ok := rawMap["window_end"].(string)
	if !ok {
		return fmt.Errorf("unable to parse window_end string")
	}
	interval, err := parseTimeInterval(rawStart, rawEnd)
	if err != nil {
		return errors.Wrap(err, "unable to parse clock time for window_start/window_end")
	}
	deploySettings.TimeInterval = &interval

	rawMaxCommits, ok := rawMap["max_commits"]
	if !ok {
		deploySettings.MaxCommits = defaultMaxCommits
	} else {
		// YAML ints are parsed as float64s
		rawMaxCommitsFloat, ok := rawMaxCommits.(float64)
		if !ok {
			return fmt.Errorf("unable to parse max_commits int")
		} else if rawMaxCommitsFloat < 1 {
			return fmt.Errorf("max_commits must be greater than 0")
		}
		deploySettings.MaxCommits = int(rawMaxCommitsFloat)
	}

	return nil
}

// TimeInterval attempts to translate a pair of strings in "HH:MM TZ" format into a TimeInterval.
// Example of one of the strings: "15:04 America/Los_Angeles"
func parseTimeInterval(start, end string) (TimeInterval, error) {
	startParts := strings.SplitN(start, " ", 2)
	if len(startParts) != 2 {
		return TimeInterval{}, fmt.Errorf("invalid format: %s", start)
	}
	startTime, err := time.Parse(hhmmLayout, startParts[0])
	if err != nil {
		return TimeInterval{}, err
	}
	startLoc, err := time.LoadLocation(startParts[1])
	if err != nil {
		return TimeInterval{}, err
	}

	endParts := strings.SplitN(end, " ", 2)
	if len(endParts) != 2 {
		return TimeInterval{}, fmt.Errorf("invalid format: %s", end)
	}
	endTime, err := time.Parse(hhmmLayout, endParts[0])
	if err != nil {
		return TimeInterval{}, err
	}
	endLoc, err := time.LoadLocation(endParts[1])
	if err != nil {
		return TimeInterval{}, err
	}

	return TimeInterval{
		StartHour:     startTime.Hour(),
		StartMinute:   startTime.Minute(),
		StartLocation: startLoc,
		EndHour:       endTime.Hour(),
		EndMinute:     endTime.Minute(),
		EndLocation:   endLoc,
	}, nil
}

func parseDataComponentsNames(data interface{}, typePrefix string) (dataComponents []DataComponent, err error) {
	deployDataComponents, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to decode data components list")
	}
	if len(typePrefix) > 0 {
		typePrefix = typePrefix + "/"
	}
	dataComponents = make([]DataComponent, len(deployDataComponents))
	for i := range deployDataComponents {
		dataComponents[i] = DataComponent{
			Name: typePrefix + deployDataComponents[i].(string),
		}
	}
	return dataComponents, nil
}

func populateDataDeployComponents(root map[string]interface{}, dataDeploy *DataDeploy) error {
	pullDeployComponents, ok := root["pulldeploy_components"]
	var err error
	var pullDeployDataComponents []DataComponent
	if ok {
		if dataMap, ok := pullDeployComponents.(map[string]interface{}); ok {
			if data, k := dataMap["data"]; k {
				pullDeployDataComponents, err = parseDataComponentsNames(data, "")
				if err != nil {
					return errors.Wrap(err, "Can't decode pulldeploy data components")
				}
			}
		} else {
			return fmt.Errorf("failed to decode data components")
		}
	}

	configsetsComponents, ok := root["configsets"]
	var configsetsDataComponents []DataComponent
	if ok {
		configsetsDataComponents, err = parseDataComponentsNames(configsetsComponents, "configset")
		if err != nil {
			return errors.Wrap(err, "Can't decode configsets")
		}
	}

	components := append(pullDeployDataComponents, configsetsDataComponents...)
	// we skip allowlisted components to avoid double processing, since we "force" inject them anyway
	for _, componentToSkip := range AllowlistedDataDeployComponents {
		for i, component := range components {
			if component == componentToSkip {
				components = append(components[:i], components[i+1:]...)
				break
			}
		}
	}

	dataDeploy.Components = components
	return nil
}

func populateDeploys(root *DeploymentNode, stages *DeploymentStages) error {
	s, err := DeploymentStagesFromRoot(root)
	if err != nil {
		return err
	}
	*stages = *s
	return nil
}

func validateDeployTarget(dt DeployTarget, stage protoDeploymentStage) error {
	if stage.isAZSpecific && dt.Facet.Type == "cron" {
		return fmt.Errorf("cron facet '%s' cannot be defined in a deploy step that targets a specific AZ", dt.Facet.Facet)
	}
	return nil
}

func parseDeploymentNode(rawRoot interface{}, m *Manifest) (*DeploymentNode, error) {
	if list, ok := rawRoot.([]interface{}); ok {
		var children []*DeploymentNode
		for _, rawSubTrunk := range list {
			n, err := parseDeploymentNode(rawSubTrunk, m)
			if err != nil {
				return nil, errors.Wrap(err, "error when processing list of stages")
			}
			children = append(children, n)
		}
		return &DeploymentNode{Children: children}, nil
	}

	var stage protoDeploymentStage
	decoder, err := newStrictDecoder(&stage)
	if err != nil {
		return nil, errors.Wrap(err, "can't build mapstructure decoder")
	}

	err = decoder.Decode(rawRoot)
	if err != nil {
		return nil, errors.Wrapf(err, "decoding structure %v failed", rawRoot)
	}
	if stage.Name == "" {
		return nil, fmt.Errorf("no name specified for deployment stage %v", stage)
	}

	az := validAzRegex.FindStringSubmatch(stage.Name)
	stage.isAZSpecific = len(az) == 3

	dts := make(map[string]DeployTarget)
	isFacetClusterTargeted := false
	if stage.Targets != nil {
		for _, target := range stage.Targets {
			targets, err := populateDeployTarget(target, m.Facets, m.FacetGroups, m.ClusterAliases)
			if err != nil {
				return nil, errors.Wrap(err, "invalid deploy target")
			}

			for dtName, dt := range targets {
				if err := validateDeployTarget(dt, stage); err != nil {
					return nil, fmt.Errorf("error parsing %s stage %s", stage.Name, err)
				}
				dts[dtName] = dt
				if dt.ClusterAlias.Name != "" {
					isFacetClusterTargeted = true
				}
			}
		}

		if stage.Kubernetes.Enabled == nil {
			return nil, fmt.Errorf("targets found for %s stage please set enabled to true on the kubernetes key", stage.Name)
		}
	}

	adminPolicies := strings.Join(stage.Kubernetes.ACL["admin_policies"], ",")
	readonlyPolicies := strings.Join(stage.Kubernetes.ACL["readonly_policies"], ",")
	ns := &Namespace{
		Name:             fmt.Sprintf("%s-%s", m.Name, stage.Name),
		Slack:            m.Slack,
		TeamEmail:        m.TeamEmail,
		AdminPolicies:    adminPolicies,
		ReadonlyPolicies: readonlyPolicies,
	}

	kstage, err := populateKubernetesStage(stage.Kubernetes)
	if err != nil {
		return nil, errors.Wrapf(err, "error decoding kubernetes stage %s", stage.Name)
	}

	if isFacetClusterTargeted {
		if !(len(kstage.ClusterTolerations) == 0 && len(kstage.ClusterLabels) == 0) {
			return nil, fmt.Errorf("cluster targeted at both facet level and at %s step", stage.Name)
		}
		for _, dt := range dts {
			// Since cron and legacyorca facets are only allowed to be deployed to specific clusters, we want a situation where the user does not have to
			// respecify the cluster label and tolerations, and in a case where they do, that they are targeting the right clusters
			if dt.ClusterAlias.Name == "" && dt.Facet.Type != FacetTypeCron && dt.Facet.Type != FacetTypeLegacyOrca {
				return nil, fmt.Errorf("cluster not targeted for facet %s in %s stage", dt.Facet.Facet, stage.Name)
			} else if dt.ClusterAlias.Name != "" {
				if dt.Facet.Type != FacetTypeCron && dt.Facet.Type != FacetTypeLegacyOrca {
					continue
				}
				clusterRole, found := dt.ClusterAlias.ClusterLabels["cluster.lyft.net/role"]
				if !found {
					return nil, fmt.Errorf("'cluster.lyft.net/role' label must be specified for cron and legacy orca facets")
				}
				if len(clusterRole) == 0 {
					return nil, fmt.Errorf("'cluster.lyft.net/role' maps to an empty key for cluster_alias %s", dt.ClusterAlias.Name)
				}
				if dt.Facet.Type == FacetTypeCron && clusterRole[0] != "omnicron" {
					return nil, fmt.Errorf("cron facet should target omnicron cluster")
				}
				if dt.Facet.Type == FacetTypeLegacyOrca && clusterRole[0] != "deploys" {
					return nil, fmt.Errorf("legacyorca facet should target deploys cluster")
				}
			}
		}
	}

	// It's an easy mistake to have a "canary" facet but forget to specify it in the list
	// of targets for the overall production deployment stage.
	_, hasCanaryFacet := m.Facets.byName[facetCanary]
	if stage.Name == stageProduction && stage.Kubernetes.Enabled != nil && *stage.Kubernetes.Enabled && hasCanaryFacet {
		hasCanaryTarget := false
		for _, target := range dts {
			if target.Facet.Canary {
				hasCanaryTarget = true
				break
			}
		}
		if !hasCanaryTarget {
			return nil, fmt.Errorf("a canary facet is defined but not targeted by stage: %s", stage.Name)
		}
	}

	var targets []DeployTarget
	for _, dt := range dts {
		targets = append(targets, dt)
	}

	if isAffectedByBake(stage) {
		if stage.BakeTimeMinutes == nil {
			stage.BakeTimeMinutes = func(i int) *int { return &i }(defaultBakeTimeMinutes)
		}
	}

	env, overlay, err := getEnvAndOverlay(stage)
	if err != nil {
		return nil, err
	}

	result := DeploymentNode{Element: &DeploymentStage{
		Name:             stage.Name,
		Legacy:           stage.Legacy,
		V3:               stage.V3,
		Automatic:        stage.Automatic,
		BakeTimeMinutes:  stage.BakeTimeMinutes,
		Kubernetes:       kstage,
		Namespace:        *ns,
		Targets:          targets,
		Links:            stage.Links,
		AvailabilityZone: stage.AvailabilityZone,
		Role:             stage.Role,
		Environment:      env,
		Overlay:          overlay,
		Schedule:         stage.Schedule,
	}}
	return &result, nil
}

// Bake times right now affect staging, canary or a production AZ
func isAffectedByBake(stage protoDeploymentStage) bool {
	return stage.Name == stageStaging || stage.Name == canaryStage || (stage.isAZSpecific && strings.Contains(stage.Name, stageProduction))
}

func getEnvAndOverlay(stage protoDeploymentStage) (string, string, error) {
	if stage.Environment == "" {
		return "", "", fmt.Errorf("environment must be provided for deployment stage: %v", stage)
	}
	if stage.Environment != "" && !validEnvs[stage.Environment] {
		return "", "", fmt.Errorf("%s: invalid environment provided: %s", stage.Name, stage.Environment)
	}
	env, overlay := stage.Environment, ""

	match := deployStep.FindStringSubmatch(stage.Name)

	groups := make(map[string]string)
	for i, n := range deployStep.SubexpNames() {
		if i != 0 && n != "" && i < len(match) {
			groups[n] = match[i]
		}
	}

	if groups["env"] == canaryStage {
		env = stageProduction
		overlay = canaryStage
	} else if validEnvs[groups["env"]] {
		env = groups["env"]
		overlay = groups["overlay"]
	}

	if stage.Environment != "" && env != "" && stage.Environment != env {
		return "", "", fmt.Errorf("%s: environment prefix in step name incongruent with specified step environment", stage.Name)
	}

	return env, overlay, nil
}

func merge(cl map[string]interface{}, output map[string][]string) {
	for k, v := range cl {
		if _, ok := output[k]; !ok {
			if vs, ok := v.(string); ok {
				output[k] = []string{vs}
			} else if vsl, ok := v.([]interface{}); ok {
				var vss []string
				for _, vsi := range vsl {
					if vs, ok := vsi.(string); ok {
						vss = append(vss, vs)
					}
				}
				output[k] = vss
			}
		} else {
			if vs, ok := v.(string); ok {
				output[k] = append(output[k], vs)
			} else if vsl, ok := v.([]interface{}); ok {
				var vss []string
				for _, vsi := range vsl {
					if vs, ok := vsi.(string); ok {
						vss = append(vss, vs)
					}
				}
				output[k] = append(output[k], vss...)
			}
		}
	}

}
