package manifest

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/robfig/cron"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
)

// UpdateStrategy signals how updates should occur in Kubernetes.
type UpdateStrategy string

const (
	// UpdateStrategyRecreate deletes old pods before creating new
	// ones. This is useful for singletons.
	UpdateStrategyRecreate UpdateStrategy = "recreate"
	// UpdateStrategyRollingUpdate slowly rolls out new pods while
	// scaling down old pods. For StatefulSets, update will be applied to all
	// Pods in the StatefulSet with respect to the StatefulSet ordering constraints.
	UpdateStrategyRollingUpdate UpdateStrategy = "rolling_update"
	// StatefulSetUpdateStrategyOnDelete disables version tracking and ordered rolling restarts
	// Pods are recreated from the StatefulSetSpec when they are manually deleted.
	StatefulSetUpdateStrategyOnDelete UpdateStrategy = "on_delete"
)

const (
	facetCanary = "canary"
)

type protoFacet struct {
	Name                    string
	Member                  string
	Labels                  map[string]string
	Type                    string
	ForceDeployment         bool
	IAM                     IAMParameters `mapstructure:"iam" json:"iam,omitempty"`
	MaxSurge                *string       `mapstructure:"max_surge"`
	MaxUnavailable          *string       `mapstructure:"max_unavailable"`
	MinReadySeconds         *int32        `mapstructure:"min_ready_seconds"`
	Sidecars                *[]string
	AdditionalSidecars      []string                `mapstructure:"additional_sidecars"`
	WatchRemoteFiles        map[string][]RemoteFile `mapstructure:"watch_remote_files"`
	Autoscaling             Autoscaling
	Schedule                string    `mapstructure:"schedule"`
	ConcurrencyPolicy       string    `mapstructure:"concurrency_policy"`
	StartingDeadlineSeconds *int64    `mapstructure:"starting_deadline_seconds"`
	EnvoyService            string    `mapstructure:"envoy_service"`
	UpdateStrategy          string    `mapstructure:"update_strategy"`
	EnvironmentFiles        *[]string `mapstructure:"environment_files"`
	Requirements            *RequirementsPolicy
	Canary                  bool
	Profiling               *profiling       `mapstructure:"profiling"`
	DisruptionBudget        DisruptionBudget `mapstructure:"disruption_budget"`
	Direct                  *Direct          `mapstructure:"direct"`
	DisableRebalance        bool             `mapstructure:"disable_rebalance"`
	Orca                    OrcaParameters   `mapstructure:"orca"`
	VolumeClaims            []VolumeClaim    `mapstructure:"volume_claims"`
	Partition               int32            `mapstructure:"partition"`
	NodePlacement           *PlacementPolicy `mapstructure:"placement"`
}

type facetResolver struct {
	groups     Groups
	containers Containers
}

type profiling struct {
	Enabled  *bool
	Language Language `mapstructure:"profile"`
	Duration int
	// validation to be performed on whether this is a float, using a string to
	// preserve decimal point accuracy
	Interval       string
	ProfilingTypes map[string]bool `mapstructure:"types"`
}

func (fr *facetResolver) resolveFacetContainers(member string) ([]*Container, error) {
	var outputContainers []*Container
	local, err := localMember(member)
	if local == "" || err != nil {
		return nil, fmt.Errorf("invalid member specified: %v", member)
	}
	group, err := fr.groups.GetByName(local)
	if err == nil {
		outputContainers = group.ListContainers()
	} else {
		container, err := fr.containers.GetByName(local)
		if err != nil {
			return nil, fmt.Errorf("could not locate any member named %v", member)
		}
		outputContainers = []*Container{container}
	}
	return outputContainers, nil
}

func populateFacets(
	facetList interface{},
	projectName string,
	groups Groups,
	containers Containers,
	envoyServices []EnvoyService,
	facets *Facets,
) error {
	list, ok := facetList.([]interface{})
	if !ok {
		return errors.Errorf("facets needs to be a list")
	}
	for _, input := range list {
		var prototype protoFacet
		var updateStrategy UpdateStrategy

		decoder, err := newStrictDecoder(&prototype)
		if err != nil {
			return errors.Wrap(err, "failed to decode facet due to decoder issue")
		}
		err = decoder.Decode(input)
		if err != nil {
			return errors.Wrap(err, "failed to decode valid facet")
		}

		if prototype.Name == "" {
			return fmt.Errorf("need to specify a facet name")
		}

		if !isValidFacetName(prototype.Name) {
			return fmt.Errorf("facet %s does not have a valid name %s", prototype.Name, validFacetName)
		}

		if strings.Contains(prototype.Name, facetCanary) && !prototype.Canary {
			return fmt.Errorf("canary facet %s needs to set the canary key to true", prototype.Name)
		}

		if prototype.Sidecars != nil && len(*prototype.Sidecars) > 0 && len(prototype.AdditionalSidecars) > 0 {
			return fmt.Errorf("cannot specify both `sidecars` and `additional_sidecars`")
		}

		maxSurge := "25%"
		if prototype.MaxSurge != nil {
			maxSurge = *prototype.MaxSurge
		}
		maxUnavailable := "0%"
		if prototype.MaxUnavailable != nil {
			maxUnavailable = *prototype.MaxUnavailable
		}
		if (maxSurge == "0%" || maxSurge == "0") && (maxUnavailable == "0%" || maxUnavailable == "0") {
			// https://github.com/kubernetes/kubernetes/blob/87eb688ec921fe20a93a25803f08f223fba3d4ee/pkg/apis/apps/validation/validation.go#L438-L439
			return fmt.Errorf("maxUnavailable cannot be 0 when maxSurge is 0")
		}

		if prototype.EnvoyService != "" {
			if !isValidSubName(prototype.EnvoyService) {
				return fmt.Errorf("facet %s does not have a valid envoyServiceName: %s",
					prototype.Name, prototype.EnvoyService)
			}

			foundService := false
			for _, service := range envoyServices {
				if service.Name == prototype.EnvoyService {
					foundService = true
					break
				}
			}
			if !foundService {
				return fmt.Errorf("facet %s refers to a non-existent envoyServiceName: %s",
					prototype.Name, prototype.EnvoyService)
			}
		}

		// cron facets cannot have an autoscaling stanza specified.
		if prototype.Type == string(FacetTypeCron) && (prototype.Autoscaling.MaxSize != nil || prototype.Autoscaling.MinSize != nil) {
			return fmt.Errorf("facet %s has type cron and has autoscaling specified", prototype.Name)
		}

		// check remote files for validity
		for envName, remoteFiles := range prototype.WatchRemoteFiles {
			if err := validateWatchedFiles(remoteFiles); err != nil {
				return fmt.Errorf("error: for facet %s: environment %s: %s", prototype.Name, envName, err)
			}
		}

		// check for valid pod disruption budget
		for envName, pdb := range prototype.DisruptionBudget.Environment {
			if err := validateDisruptionBudget(&pdb); err != nil {
				return fmt.Errorf("error: for facet %s: environment %s: %s", prototype.Name, envName, err)
			}
		}

		// check for valid placement policy
		// facets using requirements cannot specify a placement policy
		if prototype.Requirements != nil && prototype.NodePlacement != nil {
			return fmt.Errorf("facet %s specifies both a requirements and a placements stanza", prototype.Name)
		}
		if prototype.NodePlacement != nil {
			if err := validatePlacementPolicy(prototype.NodePlacement); err != nil {
				return fmt.Errorf("error: for facet %s: %s", prototype.Name, err)
			}
		}

		var outputContainers []*Container
		if prototype.Type != string(FacetTypeDirect) {
			fr := facetResolver{groups: groups, containers: containers}
			outputContainers, err = fr.resolveFacetContainers(prototype.Member)
			if err != nil {
				return err
			}
		}

		// checks for valid Ingress Policy requirement
		if prototype.Requirements != nil && prototype.Requirements.IngressPolicy != nil {
			// only service && statefulservice facets can have ingress
			if prototype.Type != string(FacetTypeService) && prototype.Type != string(FacetTypeStatefulService) {
				return fmt.Errorf("facet: %s of type: %s cannot have ingress requirement. Only service/statefulservice facets can have an ingress requirement", prototype.Name, prototype.Type)
			}

			// validate nginx annotations
			if prototype.Requirements.IngressPolicy.NginxAnnotations != nil {
				if err := validateNginxAnnotations(*prototype.Requirements.IngressPolicy.NginxAnnotations); err != nil {
					return fmt.Errorf("facet: %s has invalid Nginx annotations: %v", prototype.Name, err)
				}
			}

			// ingress requirement must include hostnames
			if len(prototype.Requirements.IngressPolicy.Hostnames) == 0 {
				return fmt.Errorf("facet: %s ingress hostnames must not be empty", prototype.Name)
			}

			// ensure hostnames lists are not empty and environment keys are correct
			for envName, hostnames := range prototype.Requirements.IngressPolicy.Hostnames {
				if _, ok := validEnvs[envName]; !ok {
					return fmt.Errorf("facet: %s invalid key %s in ingress requirement hostname field. Must be one of %s, %s", prototype.Name, envName, stageProduction, stageStaging)
				}
				if len(hostnames) == 0 {
					return fmt.Errorf("facet: %s hostnames list for environment: %s must not be empty", prototype.Name, envName)
				}
				// check hostnames match "*.lyft.net"
				for _, hostname := range hostnames {
					if !strings.HasSuffix(hostname.Name, ".lyft.net") {
						return fmt.Errorf("facet: %s hostname: %s in ingress requirement hostname field does not end with '.lyft.net'", prototype.Name, hostname.Name)
					}
					if hostname.ServicePort == 0 {
						return fmt.Errorf("facet: %s hostname: %s a service port must be specified", prototype.Name, hostname.Name)
					}

					var found bool
					for _, ctr := range outputContainers {
						for _, export := range ctr.Exports {
							if hostname.ServicePort == export.Port {
								found = true
							}
						}
					}
					if !found {
						return fmt.Errorf("facet: %s hostname: %s the service port specified is not a port exposed by the container", prototype.Name, hostname.Name)
					}

				}
			}
		}

		// don't allow singleton: true flag for facets where it doesn't make sense (due to not having replicas)
		if prototype.Requirements != nil && prototype.Requirements.Singleton {
			switch prototype.Type {
			case string(FacetTypeService):
			case string(FacetTypeWorker):
			case string(FacetTypeStatefulService):
			case string(FacetTypeDirect):
			default:
				return fmt.Errorf("facet type %s (found on %s) does not support specifying 'singleton' as a manifest requirement, and is already 'singleton' by nature. Remove this field from your manifest", prototype.Type, prototype.Name)
			}
		}

		switch prototype.Type {
		case string(FacetTypeCron):
			if _, err := cron.ParseStandard(prototype.Schedule); err != nil {
				return errors.Wrap(err, fmt.Sprintf("error parsing cron syntax: %s", prototype.Schedule))
			}
		case string(FacetTypeService), string(FacetTypeWorker):
			updateStrategy = UpdateStrategyRollingUpdate
			if prototype.UpdateStrategy == string(UpdateStrategyRecreate) {
				updateStrategy = UpdateStrategyRecreate
			}
		case string(FacetTypeJob):
		case string(FacetTypeBatch):
		case string(FacetTypeDirect):
		case string(FacetTypeLegacyDeployReleaseJob):
		case string(FacetTypeLegacyOrca):
			if prototype.Orca.IAMRole != "" {
				m, ok := AllowedOrcaOverrides[prototype.Orca.IAMRole]
				if !ok || !m[projectName] {
					return fmt.Errorf("orca IAM role override %s not allowed for project %s", prototype.Orca.IAMRole, projectName)
				}
			} else {
				prototype.Orca.IAMRole = "orca"

			}
		case string(FacetTypeStatefulService):
			// check that volume claims is valid
			for _, claim := range prototype.VolumeClaims {
				if err := validateVolumeClaim(&claim); err != nil {
					return fmt.Errorf("error: for facet %s: %s", prototype.Name, err)
				}
			}
		default:
			return fmt.Errorf("facet type is invalid for facet %v (type %v)", prototype.Name, prototype.Type)
		}

		if prototype.Type != string(FacetTypeCron) && prototype.ConcurrencyPolicy != "" {
			return fmt.Errorf(
				"facet %v should not have a concurrency policy since it is not a cron job", prototype.Name)
		}

		var concurrencyPolicy batchv1beta1.ConcurrencyPolicy
		switch strings.ToLower(prototype.ConcurrencyPolicy) {
		case strings.ToLower(string(batchv1beta1.ForbidConcurrent)):
			concurrencyPolicy = batchv1beta1.ForbidConcurrent
		case strings.ToLower(string(batchv1beta1.AllowConcurrent)):
			concurrencyPolicy = batchv1beta1.AllowConcurrent
		case strings.ToLower(string(batchv1beta1.ReplaceConcurrent)):
			concurrencyPolicy = batchv1beta1.ReplaceConcurrent
		case "":
		default:
			return fmt.Errorf("invalid concurrencyPolicy for facet %v", prototype.Name)
		}

		var startingDeadlineSeconds *int64
		if prototype.StartingDeadlineSeconds != nil {
			startingDeadlineSeconds = prototype.StartingDeadlineSeconds
		}

		// Why 37? Envoy healthcheck is configured as such: 5s initial
		// jitter + 15s initial hc + 15s confirmed hc. A pod needs to
		// be ready for at least 35 seconds for it to be considered
		// routable in the mesh.
		//
		// Why the other 2 seconds? The pods need to be distributed
		// through the envoymesh. At commit time, we see the p999 for
		// distribution is around 0.04s.
		//
		// Please see this conversation for more info:
		// https://lyft.slack.com/archives/C014PNKJRD0/p1593560269037700?thread_ts=1593189353.029900&cid=C014PNKJRD0
		minReadySeconds := int32(37)
		if prototype.MinReadySeconds != nil {
			minReadySeconds = *prototype.MinReadySeconds
		}
		f := &Facet{
			Facet:                   prototype.Name,
			Type:                    FacetType(prototype.Type),
			IAM:                     prototype.IAM,
			ForceDeployment:         prototype.ForceDeployment,
			Labels:                  prototype.Labels,
			Member:                  prototype.Member,
			Containers:              outputContainers,
			Sidecars:                prototype.Sidecars,
			AdditionalSidecars:      prototype.AdditionalSidecars,
			Autoscaling:             prototype.Autoscaling,
			WatchRemoteFiles:        prototype.WatchRemoteFiles,
			EnvoyService:            EnvoyService{Name: prototype.EnvoyService},
			UpdateStrategy:          updateStrategy,
			MaxSurge:                maxSurge,
			MaxUnavailable:          maxUnavailable,
			MinReadySeconds:         minReadySeconds,
			EnvironmentFiles:        prototype.EnvironmentFiles,
			Schedule:                prototype.Schedule,
			ConcurrencyPolicy:       concurrencyPolicy,
			StartingDeadlineSeconds: startingDeadlineSeconds,
			Requirements:            prototype.Requirements,
			Canary:                  prototype.Canary,
			Profiling:               prototype.Profiling,
			DisruptionBudget:        prototype.DisruptionBudget,
			Direct:                  prototype.Direct,
			DisableRebalance:        prototype.DisableRebalance,
			Orca:                    prototype.Orca,
			VolumeClaims:            prototype.VolumeClaims,
			Partition:               prototype.Partition,
			NodePlacement:           prototype.NodePlacement,
		}
		err = facets.Add(f)
		if err != nil {
			return errors.Wrap(err, "facet adding failed")
		}
	}
	return nil
}
