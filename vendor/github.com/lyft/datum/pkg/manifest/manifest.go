package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	provisioning "github.com/lyft/serviceprovisioner/pkg/manifest"
	"github.com/pkg/errors"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
)

var repositoryPattern = regexp.MustCompile(`@(.*):(\w+)/(.+)\.git:*(.*)`)

// HTTPGetProbe defines a path, port, and scheme for an http probe
type HTTPGetProbe struct {
	Path   string
	Port   int
	Scheme string
}

// TCPSocketProbe defines a port to use for a tcp probe
type TCPSocketProbe struct {
	Port int
}

// ExecProbe defines a command to be ran for an exec based probe
type ExecProbe struct {
	Command []string
}

// Probe is a high level struct for defining kubernetes probes
type Probe struct {
	HTTPGet   *HTTPGetProbe   `mapstructure:"http_get"`
	TCPSocket *TCPSocketProbe `mapstructure:"tcp_socket"`
	Exec      *ExecProbe      `mapstructure:"exec_probe"`

	InitialDelaySeconds int32 `mapstructure:"initial_delay_seconds"`
	PeriodSeconds       int32 `mapstructure:"period_seconds"`
	TimeoutSeconds      int32 `mapstructure:"timeout_seconds"`
	SuccessThreshold    int32 `mapstructure:"success_threshold"`
	FailureThreshold    int32 `mapstructure:"failure_threshold"`
}

// Snapshot - TODO
type Snapshot struct {
	Dump    string `json:"dump,omitempty"`
	Restore string `json:"restore,omitempty"`
}

// Volume defines a name and path for a volume claim
type Volume struct {
	Name string
	Path string
}

// ContainerExports is used to define container port exports
type ContainerExports struct {
	Name      string
	Port      int
	Protocol  string
	StartTime int `mapstructure:"start_time"`
	Optional  bool
}

// Resource are the basic set of resources kube supports
type Resource struct {
	CPU       float64 `json:"cpu,omitempty"`
	RawMemory string  `mapstructure:"memory" json:"memory,omitempty"`
	GPU       int32   `json:"gpu,omitempty"`
}

// ResourceLimits represents resource limits for CPU, memory, and GPU. Can be overridden on a per-environment basis.
type ResourceLimits struct {
	Resource    `mapstructure:",squash" json:",omitempty"`
	Environment map[string]*Resource `json:"environment,omitempty"`
}

// ResourceRequests represents resource limits for CPU, memory, and GPU. Can be overridden on a per-environment basis.
type ResourceRequests struct {
	Resource    `mapstructure:",squash" json:",omitempty"`
	Environment map[string]*Resource `json:"environment,omitempty"`
}

// gpuHelper returns a GPU value from the contents in GPU
func (r *Resource) gpuHelper() (int32, error) {
	if r.GPU < 0 {
		return r.GPU, fmt.Errorf("GPU value can't be negative, was %v", r.GPU)
	}

	return r.GPU, nil
}

// cpuHelper returns a CPU value from the contents in CPU
func (r *Resource) cpuHelper() (float64, error) {
	if r.CPU < 0 {
		return r.CPU, fmt.Errorf("CPU value can't be negative, was %v", r.CPU)
	}

	return r.CPU, nil
}

// memoryHelper returns a canonical memory value in MiB from the contents expressed in RawMemory
func (r *Resource) memoryHelper() (string, error) {
	if r.RawMemory == "" {
		return r.RawMemory, nil
	}

	rawBytes, err := strconv.ParseInt(r.RawMemory, 10, 64)
	if err == nil {
		return fmt.Sprintf("%.2fMi", float64(rawBytes)/1024.0/1024.0), nil
	}
	multiplier := 0.0
	trim := 2

	switch r.RawMemory[len(r.RawMemory)-2 : len(r.RawMemory)] {
	case "Gi":
		multiplier = 1024 * 1024 * 1024
	case "Mi":
		multiplier = 1024 * 1024
	case "Ki":
		multiplier = 1024
	}
	if multiplier == 0.0 {
		switch r.RawMemory[len(r.RawMemory)-1] {
		case 'G':
			multiplier = 1000 * 1000 * 1000
		case 'M':
			multiplier = 1000 * 1000
		case 'K':
			multiplier = 1000
		default:
			return "", fmt.Errorf("Invalid memory specifier: %v", r.RawMemory)
		}
		trim = 1
	}
	rawNumber, err := strconv.ParseFloat(r.RawMemory[0:len(r.RawMemory)-trim], 64)
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't parse memory string %v", r.RawMemory[0:len(r.RawMemory)-1])
	}
	expandedBytes := rawNumber * multiplier
	memoryInMebibytes := expandedBytes / 1024 / 1024
	stringMemoryInMebibytes := fmt.Sprintf("%.2fMi", memoryInMebibytes)

	if memoryInMebibytes <= 0 {
		return stringMemoryInMebibytes, fmt.Errorf("Memory value must be >= 0, was %v", r.RawMemory)
	}

	return stringMemoryInMebibytes, nil
}

// GetCPU for limits prioritizes env-specific settings over global settings
func (resources *ResourceLimits) GetCPU(env string) (float64, error) {
	if limit, ok := resources.Environment[env]; ok {
		return limit.cpuHelper()
	}

	return resources.Resource.cpuHelper()
}

// GetMemory for limits prioritizes env-specific settings over global settings
func (resources *ResourceLimits) GetMemory(env string) (string, error) {
	if limit, ok := resources.Environment[env]; ok {
		return limit.memoryHelper()
	}

	return resources.Resource.memoryHelper()
}

// GetGPU for limits prioritizes env-specific settings over global settings
func (resources *ResourceLimits) GetGPU(env string) (int32, error) {
	if limit, ok := resources.Environment[env]; ok {
		return limit.gpuHelper()
	}

	return resources.Resource.gpuHelper()
}

// Container represents a manifest defined container
type Container struct {
	Name            string             `json:"name,omitempty"`
	Command         string             `json:"command,omitempty"`
	Provision       string             `json:"provision,omitempty"`
	StartTime       int                `mapstructure:"start_time" json:"start_time,omitempty"`
	Exports         []ContainerExports `mapstructure:"exports" json:"exports,omitempty"`
	Mounts          []string           `json:"mounts,omitempty"`
	Clean           string             `json:"clean,omitempty"`
	Workdir         string             `mapstructure:"workdir" json:"workdir,omitempty"`
	Once            bool               `json:"once,omitempty"`
	Role            string             `json:"role,omitempty"`
	Logging         Logging            `json:"-"`
	Requests        ResourceRequests   `json:"requests,omitempty"`
	Limits          ResourceLimits     `json:"limits,omitempty"`
	SubImage        string             `mapstructure:"sub_image" json:"sub_image,omitempty"`
	ReadinessProbe  *Probe             `mapstructure:"readiness_probe" json:"readiness_probe,omitempty"`
	LivenessProbe   *Probe             `mapstructure:"liveness_probe" json:"liveness_probe,omitempty"`
	StartPhase      string             `mapstructure:"start_phase" json:"start_phase,omitempty"`
	Privileged      bool               `json:"privileged,omitempty"`
	Snapshot        Snapshot           `mapstructure:"snapshot" json:"-"`
	Network         string             `mapstructure:"network" json:"network,omitempty"`
	PID             string             `mapstructure:"pid" json:"pid,omitempty"`
	Note            string             `mapstructure:"note" json:"note,omitempty"`
	AddCapabilities []string           `mapstructure:"add_capabilities" json:"add_capabilities,omitempty"`
	Volumes         []Volume           `json:"volumes,omitempty"`

	// When adding new exported fields please add `json:"-"` or `json:",omitempty"` to ensure the fields are exported correctly
}

// Containers is a collection of Container objects referenceable by name
type Containers struct {
	byName map[string]*Container
}

// Add a container to the collection
func (c *Containers) Add(ca *Container) error {
	if _, ok := c.byName[ca.Name]; ok {
		return fmt.Errorf("container with Name %v already present", ca.Name)
	}
	if c.byName == nil {
		c.byName = make(map[string]*Container)
	}
	c.byName[ca.Name] = ca
	return nil
}

// GetByName looks up a container by its name in constant time
func (c *Containers) GetByName(name string) (*Container, error) {
	ca, ok := c.byName[name]
	if !ok {
		return nil, fmt.Errorf("no container by Name %v", name)
	}
	return ca, nil
}

// List converts the container collection into a slice, order is nondeterministic
func (c *Containers) List() []*Container {
	cr := []*Container{}
	for _, ca := range c.byName {
		cr = append(cr, ca)
	}
	return cr
}

// Copy the list of containers to a new struct
func (c *Containers) Copy() Containers {
	newByName := map[string]*Container{}
	for k, v := range c.byName {
		newByName[k] = v
	}
	return Containers{
		byName: newByName,
	}
}

////////////////////////////////

// Group defines a set of container groups
type Group struct {
	Name       string
	Containers map[string]*Container
	// RemoteMembers define Members not local to this manifest.
	// Datum can't traverse manifests in S3 yet, so this acts
	// as a placeholder to record these while loading is occurring.
	RemoteMembers []string

	// resolved is only marked when the group containers have all
	// been located in another group or from the container list.
	resolved bool
}

// ListContainers returns the list of containers in the group
func (g *Group) ListContainers() []*Container {
	o := []*Container{}
	for _, v := range g.Containers {
		o = append(o, v)
	}
	return o
}

////////////////////////////////

// Groups is a collection of group structs
type Groups struct {
	byName map[string]*Group
}

func (g *Groups) init() {
	if g.byName == nil {
		g.byName = make(map[string]*Group)
	}
}

// Add a group to the collection
func (g *Groups) Add(group *Group) error {
	g.init()
	if _, ok := g.byName[group.Name]; ok {
		return fmt.Errorf("duplicate group Name %v", group.Name)
	}

	g.byName[group.Name] = group
	return nil
}

// GetOrNew gets an existing group or makes one with the given Name
func (g *Groups) GetOrNew(name string) *Group {
	gr, ok := g.byName[name]
	if ok {
		return gr
	}

	gr = &Group{
		Name:       name,
		Containers: map[string]*Container{},
	}
	_ = g.Add(gr)
	return gr
}

// GetByName returns a group from its name - contant time
func (g *Groups) GetByName(name string) (*Group, error) {
	gr, ok := g.byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown group named %v", name)
	}
	return gr, nil
}

// List converts the collection of groups into a list
func (g *Groups) List() []*Group {
	gr := []*Group{}
	for _, gv := range g.byName {
		gr = append(gr, gv)
	}
	return gr
}

//////////////////////////////////////

// AutoscalingSetting for the resulting HPA
type AutoscalingSetting struct {
	MinSize  *int32           `mapstructure:"min_size" json:"min_size,omitempty"`
	MaxSize  *int32           `mapstructure:"max_size" json:"max_size,omitempty"`
	Criteria map[string]int64 `mapstructure:"criteria" json:"criteria,omitempty"`
}

// Autoscaling settings for different environments and/or globally
type Autoscaling struct {
	AutoscalingSetting `mapstructure:",squash"`
	Environment        map[string]AutoscalingSetting `json:"environment,omitempty"`
}

// NoSizeError is returned if no size is specified for the autoscaling settings
type NoSizeError struct{}

// NoSizeError stringer
func (n *NoSizeError) Error() string { return "no size specified" }

// GetMinSize for autoscaling prioritizing env specific settings over global level
func (a *Autoscaling) GetMinSize(env string) (int32, error) {
	if a, ok := a.Environment[env]; ok && a.MinSize != nil {
		return *a.MinSize, nil
	}
	if a.MinSize == nil {
		return 0, &NoSizeError{}
	}
	return *a.MinSize, nil
}

// GetMaxSize for autoscaling prioritizing env specific settings over global level
func (a *Autoscaling) GetMaxSize(env string) (int32, error) {
	if e, ok := a.Environment[env]; ok && e.MaxSize != nil {
		return *e.MaxSize, nil
	}
	if a.MaxSize == nil {
		return 0, &NoSizeError{}
	}
	return *a.MaxSize, nil
}

// GetCriteria gets the Criteria (map of key value pairs) for the given environment
func (a *Autoscaling) GetCriteria(env string) (map[string]int64, error) {
	if e, ok := a.Environment[env]; ok && e.Criteria != nil {
		// Need to combine the default criteria with environment specific one
		if a.Criteria != nil {
			mergedMetrics := a.Criteria

			// Merge the common and environment specific criteria
			// The environment specific criteria takes precedence (will overwrite common)
			for key, value := range e.Criteria {
				mergedMetrics[key] = value
			}
			e.Criteria = mergedMetrics
		}

		return e.Criteria, nil
	}
	return a.Criteria, nil
}

// Logging defines log format settings for a container.
type Logging struct {
	LogSpec `mapstructure:",squash" json:",omitempty"`
}

// LogFormatType defines the typical log formats.
type LogFormatType string

const (
	// LogFormatTypeUnknown is for unknown log format types.
	LogFormatTypeUnknown LogFormatType = ""
	// LogFormatTypePythonKV is for python kv format types.
	LogFormatTypePythonKV LogFormatType = "pythonkv"
	// LogFormatTypeGoJSON is for go json format types.
	LogFormatTypeGoJSON LogFormatType = "gojson"
)

// LogSpec defines the log format settings for a container.
type LogSpec struct {
	Format LogFormatType `json:"format,omitempty"`
}

// UnmarshalJSON conforms to the JSON unmarshalling interface.
func (l *LogSpec) UnmarshalJSON(b []byte) error {
	var tmpLogSpec struct {
		Format string
	}
	if err := json.Unmarshal(b, &tmpLogSpec); err != nil {
		return err
	}
	l.Format = LogFormatTypeUnknown
	switch tmpLogSpec.Format {
	case string(LogFormatTypePythonKV):
		l.Format = LogFormatTypePythonKV
	case string(LogFormatTypeGoJSON):
		l.Format = LogFormatTypeGoJSON
	}
	return nil
}

//////////////////////////////////////

// FacetType enum
type FacetType string

// Supported facet types
const (
	FacetTypeService                FacetType = "service"
	FacetTypeWorker                 FacetType = "worker"
	FacetTypeCron                   FacetType = "cron"
	FacetTypeJob                    FacetType = "job"
	FacetTypeBatch                  FacetType = "batch"
	FacetTypeLegacyDeployReleaseJob FacetType = "legacydeployandrelease"
	FacetTypeDirect                 FacetType = "direct"
	FacetTypeLegacyOrca             FacetType = "legacyorca"
	FacetTypeStatefulService        FacetType = "statefulservice"
)

// RemoteFilesByName sorts RemoteFile structs by their name.
type RemoteFilesByName []RemoteFile

func (r RemoteFilesByName) Less(i, j int) bool { return r[i].Name < r[j].Name }
func (r RemoteFilesByName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r RemoteFilesByName) Len() int           { return len(r) }

// RemoteFile represents a remote file somewhere out on the internet.
type RemoteFile struct {
	Name             string `mapstructure:"name" json:"name"`
	S3URL            string `mapstructure:"s3_url" json:"s3_url"`
	InitialRevision  string `mapstructure:"initial_revision" json:"initial_revision,omitempty"`
	DownloadLocation string `mapstructure:"download_location" json:"download_location,omitempty"`
}

// RemoteFileDeploysManagedKey represents RemoteFileDeploys managed label
const RemoteFileDeploysManagedKey = "remotefiledeploys.lyft.net/managed"

// RemoteFileDeploysFilesKey represents RemoteFileDeploysFilesLabel files label
const RemoteFileDeploysFilesKey = "remotefiledeploys.lyft.net/files"

func validateWatchedFiles(remoteFiles []RemoteFile) error {
	// ensure all the watched files have a name and a s3 url.
	remoteFileNames := make(map[string]bool)
	for i, watchedFile := range remoteFiles {
		if watchedFile.Name == "" {
			return fmt.Errorf("watched file name is blank in index %d", i)
		}
		if !isValidFileName(watchedFile.Name) {
			return fmt.Errorf("watched file name %s is invalid", watchedFile.Name)
		}
		if remoteFileNames[watchedFile.Name] {
			return fmt.Errorf("watched file name is not unique: %s", watchedFile.Name)
		}
		if watchedFile.S3URL == "" {
			return fmt.Errorf("watched s3 url is blank in index %d", i)
		}
		remoteFileNames[watchedFile.Name] = true
	}
	return nil
}

// RequirementsPolicy represents rules pertaining to special features which may not
// be available for every node
type RequirementsPolicy struct {
	// Mongo defines that a special mongo-enabled node type should be used. MLab restricts
	// access to MongoDB from anything but the primary CIDR, and Kubernetes runs in alternate CIDRs
	// to expand our IP space. This flag forces the scheduling to happen on a node which
	// has only the primary CIDR for its ENI adapters.
	Mongo bool
	// For github.com/lyft/nlbcontroller
	// This requirement annotates the service, and adds an nlbcontroller readinessGate to pods.
	NlbIngress bool
	// This is a true singleton and we want to always run one copy of this application regionally
	Singleton bool
	// This allows the pod to schedule on instance types with local nvme storage (c5d).
	// Currently, this should only be used by routingengine.
	LocalNvme bool
	// IngressPolicy contains settings that allow services to be exposed without Envoy
	IngressPolicy *IngressPolicy `mapstructure:"ingress" json:"ingress,omitempty"`
}

const nginxAnnotationKeyPrefix = "nginx.ingress.kubernetes.io"

// Hostname represents a network host which specifies a fully qualified domain name
// and a service port which is the port of the referenced service.
type Hostname struct {
	Name        string `mapstructure:"name" json:"name,omitempty"`
	ServicePort int    `mapstructure:"port" json:"port,omitempty"`
}

// IngressPolicy represents requirements and settings for service facets that
// need non-Envoy fronted ingress
type IngressPolicy struct {
	// Required, when set to true, indicates that an Ingress resource is required
	Required bool
	// Public, when set to true, indicates that the Ingress resource should be configured for the public internet
	Public *bool `json:"-,omitempty"`
	// Hostnames is a map of environments to a list of Fully-Qualified-Domain-Names (FQDNs) within the Lyft VPC
	// along with the port number it listens on.
	// When a request is issued to the specified hostname, it will be routed to this Ingress and subsequent service Pods
	// This comes directly from ELB CNAME entries.
	Hostnames map[string][]Hostname `mapstructure:"hostnames" json:"hostnames,omitempty"`
	// NginxAnnotations are Annotations that need to be applied to Ingress resouces.
	// Acceptable keys: https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/
	NginxAnnotations *map[string]string `mapstructure:"nginx_annotations" json:"nginx_annotations,omitempty"`
}

func validateNginxAnnotations(annotations map[string]string) error {
	for key := range annotations {
		if !strings.HasPrefix(key, nginxAnnotationKeyPrefix) {
			return fmt.Errorf("invalid key %v. annotation key format defined at https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/", key)
		}
	}
	return nil
}

// HasIngressPolicyRequirement determines if a given facet has a IngressPolicy requirement enabled
func HasIngressPolicyRequirement(facet *Facet) bool {
	return facet.Requirements != nil && facet.Requirements.IngressPolicy != nil && facet.Requirements.IngressPolicy.Required
}

// Facet represents a deployable component
type Facet struct {
	Facet           string            `json:"name,omitempty"`
	Type            FacetType         `json:"type,omitempty"`
	Member          string            `json:"member,omitempty"`
	ForceDeployment bool              `json:"force_deployment,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Containers      []*Container      `json:"containers,omitempty"`
	Sidecars        *[]string         `json:"sidecars,omitempty"`
	// AdditionalSidecars allows a user to specify sidecars they want in addition to the default set.
	// Specifying both Sidecars and AdditionalSidecars is an error.
	AdditionalSidecars []string         `json:"additional_sidecars,omitempty"`
	Autoscaling        Autoscaling      `json:"autoscaling,omitempty"`
	EnvoyService       EnvoyService     `json:"-"`
	DisruptionBudget   DisruptionBudget `mapstructure:"disruption_budget" json:"-"`
	DisableRebalance   bool             `json:"disable_rebalance,omitempty"`
	VolumeClaims       []VolumeClaim    `json:"volume_claims,omitempty"`

	// IAM contains the IAM settings for this specific facet.
	IAM IAMParameters `json:"-"`

	// WatchRemoteFiles is a map of environments to a list of remote
	// files for which, when their contents change, should trigger
	// rollouts.
	WatchRemoteFiles map[string][]RemoteFile `mapstructure:"watch_remote_files" json:",omitempty"`

	// UpdateStrategy is the strategy in which we should update a
	// deployment.
	UpdateStrategy UpdateStrategy `json:"update_strategy,omitempty"`

	// MaxSurge is the maximum surge percentage (i.e. a string such as
	// 25%) of the Deployment.
	MaxSurge string `json:"max_surge,omitempty"`

	// MaxUnavailable specifies the maximum number of Pods that can be
	// unavailable during the update process. The value can be an absolute
	// number (e.g. 5) or a percentage of desired Pods (e.g. 10%)
	MaxUnavailable string `json:"max_unavailable,omitempty"`

	// MinReadySeconds is the minimum amount of seconds that a Pod
	// must be ready before it is considered "available" by the
	// Kubernetes deployment; Deployments progress only when pods are
	// "available"
	MinReadySeconds int32 `json:"min_ready_seconds,omitempty"`

	// Define a list of environments to load. If this is not set
	// a default set of loading rules will be used.
	EnvironmentFiles *[]string `json:"environment_files,omitempty"`

	// Schedule is a valid cron expressions for cron FacetType
	Schedule string `json:"schedule,omitempty"`

	// ConcurrencyPolicy specifies how to treat concurrent executions of a job started by the cronjob.
	// It should be limited to "Allow", "Forbid" and "Replace"
	ConcurrencyPolicy batchv1beta1.ConcurrencyPolicy `json:"concurrency_policy,omitempty"`

	// StartingDeadlineSeconds specifies how many seconds into the past the CronJob controller looks in order to
	// count how many jobs could not be scheduled for a CronJob deployment.
	StartingDeadlineSeconds *int64 `json:"starting_deadline_seconds,omitempty"`

	// Canary is a boolean which sets a number of down-steam lyft specific behaviors
	Canary bool `json:"canary,omitempty"`

	// Requirements defines where to schedule resources within a cluster
	Requirements *RequirementsPolicy `json:"requirements,omitempty"`

	// Direct defines configuration for a type=direct facet.
	Direct *Direct `json:"direct,omitempty"`

	// Override profiling flags
	Profiling *profiling `json:"profiling,omitempty"`

	// Orca facet specific data
	Orca OrcaParameters `json:"orca,omitempty"`

	Volumes []Volume `json:"-"`

	// Partition indicates the ordinal at which the StatefulSet should be partitioned.
	Partition int32 `json:"-"`

	// NodePlacement specifies NodeSelector and taint tolerations for the placement of the facet
	NodePlacement *PlacementPolicy `json:"placement,omitempty"`

	// identifier for cluster alias
	ClusterAlias ClusterAlias

	// Facet is deployed for service offloading testing purposes.
	IsOffloaded bool

	// When adding new exported fields please add `json:"-"` or `json:",omitempty"` to ensure the fields are exported correctly
}

// PlacementPolicy allows passing in arbitrary values that will be applied to facet NodeSelector and tolerations
// to allow workloads to be more selective of where they will run
type PlacementPolicy struct {
	Selector    map[string]string
	Tolerations map[string]string
}

func validatePlacementPolicy(p *PlacementPolicy) error {
	if p.Selector == nil || p.Tolerations == nil {
		return fmt.Errorf("placement.selector and placement.tolerations cannot be nil")
	}
	if err := validateTolerations(p.Selector); err != nil {
		return err
	}
	if err := validateTolerations(p.Tolerations); err != nil {
		return err
	}
	return nil
}

func validateTolerations(tolerations map[string]string) error {
	for k, v := range tolerations {
		if !validLabelKey.MatchString(k) {
			return fmt.Errorf("invalid key %v", k)
		}
		if !validLabelValue.MatchString(v) {
			return fmt.Errorf("invalid value %v in key %v", v, k)
		}
	}
	return nil
}

// IAMParameters is used to override the IAM Role that will be assumed by default.
type IAMParameters struct {
	// Roles maps an environment name to an IAM Role's short name, or
	// ARN.
	Roles map[string]string `mapstructure:"roles" json:"roles,omitempty"`
}

// OrcaParameters captures the optional tuning parameters for a LegacyOrca facet
type OrcaParameters struct {
	Location    string `mapstructure:"location" json:"name:location,omitempty"`
	Region      string `mapstructure:"region" json:"name:region,omitempty"`
	AccountName string `mapstructure:"account_name" json:"name:account_name,omitempty"`
	SubArgs     string `mapstructure:"sub_args" json:"name:sub_args,omitempty"`
	IAMRole     string `mapstructure:"iam_role" json:"name:iam_role,omitempty"`
	Credentials bool   `mapstructure:"credentials" json:"name:credentials,omitempty"`
}

// AccountID translates an account name into an account ID. Zimride is the default.
func (o OrcaParameters) AccountID() string {
	switch o.AccountName {
	case "sandbox":
		return "501253315745"
	case "security":
		return "036177710368"
	case "lyftqubole":
		return "708015885778"
	case "lyftk8stest":
		return "070647025895"
	case "lyft-level5":
		return "510079712231"
	case "lyft-level5-dev":
		return "809340713038"
	case "zimride":
		return "173840052742"
	default:
		return "173840052742"
	}
}

// Inherit the facet with a new cluster alias
func (f *Facet) Inherit(clusterAlias ClusterAlias) *Facet {
	newFacet := *f
	newFacet.ClusterAlias = clusterAlias
	return &newFacet
}

// DefaultFacet is an empty service facet
var DefaultFacet = Facet{
	Labels: map[string]string{},
	Type:   "service",
}

// Facets is a collection of facets
type Facets struct {
	byName map[string]*Facet
}

// Add a facet to the collection
func (s *Facets) Add(f *Facet) error {
	if s.byName == nil {
		s.byName = make(map[string]*Facet)
	}

	if _, ok := s.byName[f.Facet]; ok {
		return fmt.Errorf("duplicate facet name %v", f.Facet)
	}

	s.byName[f.Facet] = f
	return nil

}

// List converts the collection of facets to a list
func (s *Facets) List() []*Facet {
	fs := []*Facet{}
	for _, f := range s.byName {
		fs = append(fs, f)
	}
	return fs
}

// GetByName looks up a facet by its name in constant time
func (s *Facets) GetByName(name string) (*Facet, error) {
	f, ok := s.byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown facet named %v", name)
	}
	return f, nil
}

////////////////////////////////////

// FacetGroup represents a group of facets that can be targeted together
type FacetGroup struct {
	Name    string
	Members map[string]*Facet

	// resolved is set to true when all the facets have been resolved or groups expanded
	resolved bool
}

// FacetGroups is a collection of facet groups
type FacetGroups struct {
	byName map[string]*FacetGroup
}

// Add a facet to the collection
func (fg *FacetGroups) Add(f *FacetGroup) error {
	if fg.byName == nil {
		fg.byName = make(map[string]*FacetGroup)
	}

	if _, ok := fg.byName[f.Name]; ok {
		return fmt.Errorf("duplicate facet group name %v", f.Name)
	}

	fg.byName[f.Name] = f
	return nil

}

// GetOrNew gets an existing facet group or makes one with the given Name
func (fg *FacetGroups) GetOrNew(name string) *FacetGroup {
	gr, ok := fg.byName[name]
	if ok {
		return gr
	}

	gr = &FacetGroup{
		Name:    name,
		Members: map[string]*Facet{},
	}
	_ = fg.Add(gr)
	return gr
}

// GetByName looks up a facet group by its name in constant time
func (fg *FacetGroups) GetByName(name string) (*FacetGroup, error) {
	f, ok := fg.byName[name]
	if !ok {
		return nil, fmt.Errorf("unknown facet group named %v", name)
	}
	return f, nil
}

////////////////////////////////////

// Repository references meta data for a git repository
type Repository struct {
	Host    string
	Org     string
	Name    string
	SubPath string
}

// URL is the http url for the repository
func (r *Repository) URL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Org, r.Name)
}

////////////////////////////////////

// Languages is a list of language definitions
type Languages []Language

// HasLanguage checks if the given language exists in languages
func (l Languages) HasLanguage(lang Language) bool {
	for _, c := range l {
		if c == lang {
			return true
		}
	}
	return false
}

// Supported Languages
const (
	LanguageGo     = "go"
	LanguagePython = "python"
	LanguageNone   = ""
	ImageService   = "service"
	ImageLibrary   = "library"
	ImageData      = "data"
	ImageTool      = "tool"
	ImageLambda    = "lambda"
)

// Language is a programming language
type Language string

// ImageType is a docker image type
type ImageType string

////////////////////////////////////

// Params is an object for builder specific configuration
// This struct includes all the possible values of parameters
// for different builders.
type Params struct {
	Builder    string   `mapstructure:"builder" json:"-"`
	SubImage   string   `mapstructure:"sub_image" json:"sub_image,omitempty"`
	Packages   []string `mapstructure:"packages" json:"packages,omitempty"`
	Dockerfile string   `mapstructure:"dockerfile" json:"dockerfile,omitempty"`
	BuildArgs  []string `mapstructure:"buildargs" json:"buildargs,omitempty"`
	Sources    []Source `mapstructure:"sources" json:"sources,omitempty"`
}

// SingleBuilder is the builder type for a single builder
type SingleBuilder struct {
	Name   string `json:"name"`
	Params Params `mapstructure:"params" json:"params,omitempty"`
}

// SubImageNames returns the sub_image value of the builder.
// If the builder does not specify a sub_image, the result will be equivalent to []string{""}.
func (sb SingleBuilder) SubImageNames() []string {
	return []string{sb.Params.SubImage}
}

// Builders returns a slice of this builder
func (sb SingleBuilder) Builders() []Params {
	return []Params{sb.Params}
}

// BuilderList is a list of builder settings
type BuilderList []SingleBuilder

// BuilderParams is used as input for a builder
type BuilderParams struct {
	Builders BuilderList `json:"builders"`
}

// MultiBuilder defines a tyoe that has multiple builders
type MultiBuilder struct {
	Name   string        `json:"name"`
	Params BuilderParams `mapstructure:"params" json:"params,omitempty"`
}

// SubImageNames returns the sub_image values of the component builders.
// If a builder does not specify a sub_image, its contribution will be "".
func (mb MultiBuilder) SubImageNames() []string {
	ret := []string{}
	seen := make(map[string]bool)
	for _, builder := range mb.Params.Builders {
		for _, subImageName := range builder.SubImageNames() {
			if seen[subImageName] {
				continue
			}
			seen[subImageName] = true
			ret = append(ret, builder.SubImageNames()...)
		}
	}
	return ret
}

// Builders returns a list of all known builder parameters
func (mb MultiBuilder) Builders() []Params {
	var builders []Params
	for _, builder := range mb.Params.Builders {
		builders = append(builders, builder.Builders()...)
	}
	return builders
}

// Builder is an interface that is implemented by both
// SingleBuilder and MultiBuilder
type Builder interface {
	populate(interface{}) error
	SubImageNames() []string
	Builders() []Params
}

////////////////////////////////////

// V3DeploymentStage is used to explicitly enable or disable a v3 release for a stage.
type V3DeploymentStage struct {
	Enabled bool
}

////////////////////////////////////

// KubernetesDeploymentStage is the final definition used to generate the final k8s spec
type KubernetesDeploymentStage struct {
	Enabled bool
	// ClusterLabels is a set of labels to match against. Keys are
	// the label key, value is a specific value to match against. An empty
	// value matches against all keys for that cluster
	ClusterLabels map[string][]string `mapstructure:"clusterLabels"`

	// ClusterTolerations is a set of taints specified on the cluster this deployment will
	// tolerate.
	ClusterTolerations map[string][]string `mapstructure:"cluster_tolerations"`

	// Allow a sub-path to be referenced for deployments of explicit manifests
	Path string

	// Pre-configured ACL policies for RBAC
	ACL map[string][]string `mapstructure:"acl"`

	// Allow overriding a mode - e.g. direct or synthesized
	Mode string

	// Allow disabling any interpolation
	DisableInterpolation bool `mapstructure:"disable_interpolation"`

	// Variables to pass to interpolation when building variables. Can override
	// existing interpolation varibales which are used by k8sdeploy. Ignored
	// by k8sdeploy for synthesis mode.
	InterpolationVars map[string]string `mapstructure:"interpolation_vars"`
}

////////////////////////////////////

// DeployLocation defines the region for the deployment
type DeployLocation struct {
	Region string
}

////////////////////////////////////

// DeployTarget ties a facet to one or more locations
type DeployTarget struct {
	Facet        *Facet
	ClusterAlias ClusterAlias
	Locations    []DeployLocation
}

////////////////////////////////////

// Namespace meta data
type Namespace struct {
	Name             string
	Slack            string
	TeamEmail        string
	AdminPolicies    string
	ReadonlyPolicies string
}

// DeploymentLink meta data
type DeploymentLink struct {
	Name string `mapstructure:"name"`
	Text string `mapstructure:"text"`
	URL  string `mapstructure:"url"`
}

// DeploymentStage is a distinct stage of a deployment
type DeploymentStage struct {
	Name             string
	Legacy           bool
	V3               *V3DeploymentStage
	Kubernetes       KubernetesDeploymentStage
	Automatic        bool
	BakeTimeMinutes  *int
	Namespace        Namespace
	Links            []DeploymentLink
	AvailabilityZone *string
	Role             *string
	Environment      string
	Overlay          string

	// Old-school scheduled deploys in Jenkins
	Schedule *string
	// Targets is consumed by locating or overriding a previously
	// specified facet
	Targets []DeployTarget
}

// DeploymentNode container for constructing the deployment tree
type DeploymentNode struct {
	Children []*DeploymentNode
	Element  *DeploymentStage
}

// List converts the collection of deployments to a list
func (d *DeploymentNode) List() []*DeploymentStage {
	var results []*DeploymentStage
	if d == nil {
		return results
	}
	if d.Element != nil {
		results = append(results, d.Element)
	}
	for _, child := range d.Children {
		results = append(results, child.List()...)
	}
	return results
}

// GetStagesInSteps organizes the deployment stages in step order.
// This assumes that the DeploymentNode that it's acting on is the root
// node, whose children may directly contain the DeploymentStage itself
// if it's a singleton or a wrapper DeploymentNode whose Children are singletons
// that directly contain the DeploymentStage.
func (d *DeploymentNode) GetStagesInSteps() [][]*DeploymentStage {
	var results [][]*DeploymentStage
	if d == nil {
		return results
	}
	for _, child := range d.Children {
		var step []*DeploymentStage
		// If the direct element is nil then let's look at its children
		if child.Element == nil {
			for _, grandchild := range child.Children {
				step = append(step, grandchild.Element)
			}
		} else {
			step = append(step, child.Element)
		}
		results = append(results, step)
	}
	return results
}

// String is useful for debugging
func (d DeploymentNode) String() string {
	return strings.TrimSpace(d.string(0))
}

func (d DeploymentNode) string(depth int) string {
	var prefix, s string
	for i := 0; i < depth; i++ {
		prefix += "\t"
	}
	var name string
	if d.Element != nil {
		name = d.Element.Name
	}
	s += fmt.Sprintf("%sElement Name: %v\n", prefix, name)
	for _, child := range d.Children {
		s += child.string(depth + 1)
	}
	return s
}

////////////////////////////////////

// DataComponent provides simple helpers for manipulating the name of a data component in k8s
type DataComponent struct {
	Name string
}

// VolumePath converts the name of a data component into a conforming k8s string
func (d DataComponent) VolumePath() string {
	return strings.Replace(d.Name, "-", "_", -1)
}

// VolumeName converts the name of a data component into a conforming k8s string
func (d DataComponent) VolumeName() string {
	volumeName := strings.Replace(d.Name, "/", "-", -1)
	return strings.Replace(volumeName, "_", "-", -1)
}

// AllowlistedDataDeployComponents represents the list of components
// that are pulled regardless of whether they are required in the manifest
var AllowlistedDataDeployComponents = []DataComponent{
	{Name: "runtime-data"},
}

// CachedDataDeployComponents the list of components pulled on every node
// before https://jira.lyft.net/browse/DEPLOY-960 is implemented
var CachedDataDeployComponents = []DataComponent{
	{Name: "marketplaceconfig"},
	{Name: "regioncfgdata"},
	{Name: "runtime-data"},
	{Name: "translationdata"},
}

// IsCachedDataDeployComponent determines if the provided
// DataComponent is pre-seeded/cached on the host instead of locally
// within the pod.
//
// A few data deploy components, specified in
// CachedDataDeployComponents, are cached on Nodes instead of inside
// of Pods.
func IsCachedDataDeployComponent(d DataComponent) bool {
	for _, cached := range CachedDataDeployComponents {
		if cached.Name == d.Name {
			return true
		}
	}
	return false
}

////////////////////////////////////

// DataDeploy is a collection of data components
type DataDeploy struct {
	Components []DataComponent
}

// String representation of a DataDeploy
func (d DataDeploy) String() string {
	componentNames := make([]string, len(d.Components))
	for i, component := range d.Components {
		componentNames[i] = component.Name
	}
	return strings.Join(componentNames, " ")
}

// CachedComponents filters for the data deploys that are expected
// to be cached on the node as handled by the runtimepull daemonset.
// Notably includes all allowlisted data deploy components even if unpresent in the DataDeploy receiver.
func (d DataDeploy) CachedComponents() []DataComponent {
	filtered := []DataComponent{}
	seen := map[string]bool{}
	for _, component := range AllowlistedDataDeployComponents {
		filtered = append(filtered, component)
		seen[component.Name] = true
	}
	for _, component := range d.Components {
		if IsCachedDataDeployComponent(component) && !seen[component.Name] {
			filtered = append(filtered, component)
			seen[component.Name] = true
		}
	}
	return filtered
}

// UncachedComponents filters for the data deploys that are expected
// to be pulled by the runtimepull sidecar.
func (d DataDeploy) UncachedComponents() []DataComponent {
	filtered := make([]DataComponent, 0)
	for _, component := range d.Components {
		if !IsCachedDataDeployComponent(component) {
			filtered = append(filtered, component)
		}
	}
	return filtered
}

// ParseDataDeploy attempts to parse a DataDeploy object from the given string
func ParseDataDeploy(s string) DataDeploy {
	if len(s) == 0 {
		return DataDeploy{}
	}
	dataDeployComponentNames := strings.Split(s, " ")
	dataComponents := make([]DataComponent, len(dataDeployComponentNames))
	for i, componentName := range dataDeployComponentNames {
		dataComponents[i] = DataComponent{
			Name: componentName,
		}
	}

	return DataDeploy{
		Components: dataComponents,
	}
}

////////////////////////////////////

// DeploymentStages is a collection of distinct deployment stages and their order
type DeploymentStages struct {
	byName         map[string]*DeploymentStage
	orderedDeploys []*DeploymentStage
	root           *DeploymentNode
}

// DeploymentStagesFromRoot converts a deployment tree into a linear list in the proper order
func DeploymentStagesFromRoot(root *DeploymentNode) (*DeploymentStages, error) {
	ds := DeploymentStages{root: root}
	for _, stage := range root.List() {
		if err := ds.add(stage); err != nil {
			return nil, err
		}
	}
	return &ds, nil
}

func (d *DeploymentStages) init() {
	if d.byName == nil {
		d.byName = map[string]*DeploymentStage{}
	}
}

func (d *DeploymentStages) add(stage *DeploymentStage) error {
	d.init()
	if _, ok := d.byName[stage.Name]; ok {
		return fmt.Errorf("duplicate deployment stage %v", stage.Name)
	}
	d.byName[stage.Name] = stage
	d.orderedDeploys = append(d.orderedDeploys, stage)
	return nil
}

// GetByName looks up a deployment stage by its name in constant time
func (d *DeploymentStages) GetByName(name string) (*DeploymentStage, error) {
	v, ok := d.byName[name]
	if !ok {
		return nil, fmt.Errorf("deployment stage %v is not available", name)
	}
	return v, nil
}

// Root of the deployment stages
func (d *DeploymentStages) Root() *DeploymentNode {
	if d.root == nil {
		d.root = &DeploymentNode{}
	}
	return d.root
}

// List converts the collection into a list
func (d *DeploymentStages) List() []*DeploymentStage {
	d.init()
	return d.orderedDeploys
}

/////////////////////////////////////////////

// EnvoyService name
type EnvoyService struct {
	Name string
}

// EnvoyHostSettings internal and external hosts for each service
// key: envoy service, value: denotes tls settings (not used) or nil
type EnvoyHostSettings struct {
	InternalHosts map[string]struct{} `mapstructure:"internal_hosts" json:"internal_hosts,omitempty"`
	ExternalHosts map[string]struct{} `mapstructure:"external_hosts" json:"external_hosts,omitempty"`
}

// EnvoySettings wrapper for envoy sidecar knobs
type EnvoySettings struct {
	Services         []EnvoyService
	CommonSettings   EnvoyHostSettings `mapstructure:"common_settings" json:"common_settings,omitempty"`
	OverrideSettings []struct {
		Services []EnvoyService
		Settings EnvoyHostSettings
	} `mapstructure:"override_settings" json:"override_settings,omitempty"`
}

// Envoy profile name and related services
type Envoy struct {
	Profile  string
	Settings EnvoySettings
}

/////////////////////////////////////////////

// Orchestration top level key that defines service's orchestration workflow
type Orchestration struct {
	Policies Policies
	Roots    TerraformRoots
}

// TerraformRoots is a collection
type TerraformRoots struct {
	byName map[string]*TerraformRoot
}

// TerraformRoot defines an individual root with a name, source directory and list of policies to check against.
type TerraformRoot struct {
	Name             string
	Directory        string
	WhenModified     []string
	TerraformVersion string
	Policies         []*Policy
}

// Policies containt a collection of policies
type Policies struct {
	byName map[string]*Policy
}

// Policy is a policy
// Each policy has Type that defines if its mandatory or advisory
// It has a Source that point to policy definition
// And a list of policy owners. There TomRoles that can approve a pending policy.
type Policy struct {
	Name   string
	Type   PolicyType
	Source PolicySource
	Owners []*PolicyOwner
}

// PolicySource defines the source of the policy.
// This could be local or remote/shared.
type PolicySource struct {
	Type PolicySourceType
	Path string
}

// PolicyType enum
type PolicyType string

// Supported policy types
const (
	PolicyTypeHardMandatory PolicyType = "hard-mandatory"
	PolicyTypeSoftMandatory PolicyType = "soft-mandatory"
)

// OwnerType enum
type OwnerType string

// Supported owner types
const (
	OwnerTypeUserName OwnerType = "username"
)

// PolicySourceType defines policy source types local or shared
type PolicySourceType string

// Supported policy source types
const (
	PolicySourceLocal  PolicySourceType = "local"
	PolicySourceGithub PolicySourceType = "github"
)

// PolicyOwner defines roles that are permitted to approve mandatory policies.
type PolicyOwner struct {
	Type OwnerType
	Name string
}

// Add an policy to the collection
func (p *Policies) Add(policy *Policy) error {
	if p.byName == nil {
		p.byName = make(map[string]*Policy)
	}

	if _, ok := p.byName[policy.Name]; ok {
		return errors.Errorf("duplicate policy name %v", policy.Name)
	}

	p.byName[policy.Name] = policy

	return nil
}

// List converts collection of policies to a list
func (p *Policies) List() []*Policy {
	opaPolicies := []*Policy{}
	for _, policy := range p.byName {
		opaPolicies = append(opaPolicies, policy)
	}

	return opaPolicies
}

// GetByName looks up policy by name in constant time
func (p *Policies) GetByName(name string) (*Policy, error) {
	policy, ok := p.byName[name]
	if !ok {

		return nil, errors.Errorf("unknown policy named %v", name)
	}
	return policy, nil
}

// Add a terraform root to the collection
func (tfr *TerraformRoots) Add(tfRoot *TerraformRoot) error {
	if tfr.byName == nil {
		tfr.byName = make(map[string]*TerraformRoot)
	}

	if _, ok := tfr.byName[tfRoot.Name]; ok {
		return errors.Errorf("duplicate Terraform Root name %v", tfRoot.Name)
	}

	tfr.byName[tfRoot.Name] = tfRoot

	return nil
}

// List converts collection of terraform roots to a list
func (tfr *TerraformRoots) List() []*TerraformRoot {
	terraformRoots := []*TerraformRoot{}
	for _, root := range tfr.byName {
		terraformRoots = append(terraformRoots, root)
	}

	return terraformRoots
}

// GetByName looks up terraform root by name in constant time
func (tfr *TerraformRoots) GetByName(name string) (*TerraformRoot, error) {
	tfRoot, ok := tfr.byName[name]
	if !ok {
		return nil, errors.Errorf("unknown Terraform Root named %v", name)
	}
	return tfRoot, nil
}

/////////////////////////////////////////////

// Direct represents the configuration of a direct facet.
type Direct struct {
	Paths []string `json:"paths,omitempty"`
}

// DisruptionBudget describes a PodDisruptionBudget
type DisruptionBudget struct {
	DisruptionBudgetSetting `mapstructure:",squash" json:",omitempty"`
	Environment             map[string]DisruptionBudgetSetting `json:",omitempty"`
}

// DisruptionBudgetSetting describes the settings for a PodDisruptionBudget
type DisruptionBudgetSetting struct {
	MinAvailablePercent   *string `mapstructure:"min_percent_available" json:",omitempty"`
	MaxUnavailablePercent *string `mapstructure:"max_percent_unavailable" json:",omitempty"`
}

// NoBudgetError is returned if no value is specified for the disruption budget settings
type NoBudgetError struct{}

// NoBudgetError stringer
func (n *NoBudgetError) Error() string { return "no budget specified" }

// GetMinAvailablePercent for PDB prioritizing env specific settings over global level
func (d *DisruptionBudget) GetMinAvailablePercent(env string) (string, error) {
	if d, ok := d.Environment[env]; ok && d.MinAvailablePercent != nil {
		return *d.MinAvailablePercent, nil
	}
	return "", nil
}

// GetMaxUnavailablePercent for PDB prioritizing env specific settings over global level
func (d *DisruptionBudget) GetMaxUnavailablePercent(env string) (string, error) {
	if d, ok := d.Environment[env]; ok && d.MaxUnavailablePercent != nil {
		return *d.MaxUnavailablePercent, nil
	}
	return "", nil
}

func validateDisruptionBudget(d *DisruptionBudgetSetting) error {
	// either MinAvailable or MaxAvailable must be specified, but not both
	if d.MaxUnavailablePercent == nil && d.MinAvailablePercent == nil {
		return &NoBudgetError{}
	}
	if d.MaxUnavailablePercent != nil && d.MinAvailablePercent != nil {
		return fmt.Errorf("either MaxUnavailablePercent or MinAvailablePercent must be set")
	}
	return nil
}

/////////////////////////////////////////////

// VolumeClaim represents the configuration of a PersistentVolumeClaim
type VolumeClaim struct {
	Name             string   `mapstructure:"name" json:"name,omitempty"`
	MountPath        string   `mapstructure:"mount_path" json:"mount_path,omitempty"`
	MinSize          string   `mapstructure:"min_size" json:"min_size,omitempty"`
	MaxSize          string   `mapstructure:"max_size" json:"max_size,omitempty"`
	AccessModes      []string `mapstructure:"access_modes" json:"access_modes,omitempty"`
	StorageClassName string   `mapstructure:"storage_class" json:"storage_class,omitempty"`
}

// InvalidVolumeClaimError is returned if the VolumeClaim config is invalid
type InvalidVolumeClaimError struct{}

// InvalidVolumeClaimError stringer
func (n *InvalidVolumeClaimError) Error() string { return "invalid volume claim specified" }

func validateVolumeClaim(v *VolumeClaim) error {
	if v.MaxSize == "" || v.MinSize == "" || v.Name == "" || v.MountPath == "" || v.AccessModes == nil {
		return &InvalidVolumeClaimError{}
	}
	return nil
}

/////////////////////////////////////////////

// DeployNotificationOverrides defines a slack override for deploy notifications
type DeployNotificationOverrides struct {
	Slack    string
	Disabled bool
}

// NotificationOverrides for slack
type NotificationOverrides struct {
	Deploy DeployNotificationOverrides `mapstructure:"deploy"`
}

/////////////////////////////////////////////

// DeploySettings for configuring deploys and practices.
type DeploySettings struct {
	// TimeInterval may not be present in all DeploySettings.
	// Users are expected to check for `nil` before use.
	// If present, all subfields will be present.
	TimeInterval *TimeInterval
	// The maximum number of commits per deployment.
	// Auto deploys will not happen if the number of commits would exceed this.
	// Deploybot will require '/force' to override this.
	MaxCommits int
}

// TimeInterval represents an date-agnostic window in time (e.g. 9a America/New_York to 5p America/Los_Angeles)
type TimeInterval struct {
	StartHour     int
	StartMinute   int
	StartLocation *time.Location
	EndHour       int
	EndMinute     int
	EndLocation   *time.Location
}

/////////////////////////////////////////////

// Manifest represents the root keys in the main manifest
type Manifest struct {
	Name                      string
	Slack                     string
	PagerdutyEscalationPolicy string `mapstructure:"pagerduty_escalation_policy"`
	TeamEmail                 string `mapstructure:"team_email"`
	Description               string
	// ServiceTier may not be present in all manifests as not all projects are services.
	// Users are expected to check for `nil` before use.
	ServiceTier           *int `mapstructure:"service_tier"`
	Repository            string
	Credentials           bool
	Disabled              bool
	Languages             Languages              `mapstructure:"languages" json:"languages,omitempty"`
	ImageType             ImageType              `mapstructure:"image_type" json:"image_type,omitempty"`
	NotificationOverrides *NotificationOverrides `mapstructure:"notification_overrides"`
	Configsets            []string               `mapstructure:"configsets" json:"configsets,omitempty"`
	Envoy                 *Envoy
	Orchestration         Orchestration
	Builder               Builder                   `mapstructure:"builder"`
	Provisioning          provisioning.Provisioning `mapstructure:"provisioning"`

	// KubernetesOnly signifies that this project only uses
	// Kubernetes.
	KubernetesOnly *bool `mapstructure:"kubernetes_only"`

	// the following types are not parsed straight by mapstructure in one pass and are built up
	// by incremental parsing.

	Containers     Containers `mapstructure:"containers" json:"containers,omitempty"`
	Groups         Groups     `mapstructure:"groups" json:"groups,omitempty"`
	Deployments    DeploymentStages
	DeploySteps    [][]*DeploymentStage
	DeploySettings DeploySettings `mapstructure:"deploy_settings"`
	Facets         Facets
	FacetGroups    FacetGroups `mapstructure:"facet_groups"`
	Data           DataDeploy
	ClusterAliases ClusterAliases `mapstructure:"cluster_aliases" json:"cluster_aliases,omitempty"`

	RawManifest []byte `json:"-"`

	// When adding new exported fields please add `json:"-"` or `json:",omitempty"` to ensure the fields are exported correctly
}

// UnmarshalJSON conforms to the json.Unmarshal interface.
func (m *Manifest) UnmarshalJSON(b []byte) error {
	// we use the generic "interface" because mapstructure essentially
	// does all the work.
	var gen map[string]interface{}
	if err := json.Unmarshal(b, &gen); err != nil {
		return errors.Wrap(err, "manifest parse failed")
	}
	if err := populate(m, gen); err != nil {
		return err
	}
	m.RawManifest = b
	return nil
}

// ParseRepository returns a parsed struct representing the repository
// string in a manifest.
func ParseRepository(repo string) (Repository, error) {
	var r Repository
	groups := repositoryPattern.FindStringSubmatch(repo)
	if len(groups) < 4 {
		return r, fmt.Errorf("invalid repository: %s", repo)
	}

	r.Host, r.Org, r.Name = groups[1], groups[2], groups[3]
	// Mono repo
	if len(groups) > 4 {
		r.SubPath = groups[4]
	}
	return r, nil
}

// ParseRepository from the manifest
func (m Manifest) ParseRepository() (Repository, error) {
	return ParseRepository(m.Repository)
}

// CodeRoot returns a project's expected working directory.
func (m *Manifest) CodeRoot() (string, error) {
	repo, err := m.ParseRepository()
	if err != nil {
		return "", errors.Wrap(err, "error parsing manifest")
	}
	var hasGo bool
	for _, language := range m.Languages {
		if language == LanguageGo {
			hasGo = true
			break
		}
	}
	prefix := "/code"
	if hasGo {
		prefix = path.Join("/go", "src", repo.Host, "lyft")
	}

	return path.Join(prefix, repo.Name, repo.SubPath), nil
}

// DeploySlack returns the correct slack notification taking into account overrides
func (m *Manifest) DeploySlack() string {
	if m.NotificationOverrides != nil && m.NotificationOverrides.Deploy.Slack != "" {
		return m.NotificationOverrides.Deploy.Slack
	}

	return m.Slack
}

// DeployNotificationsDisabled returns whether or not the manifest has
// deploy notifications disabled.
func (m *Manifest) DeployNotificationsDisabled() bool {
	if m.NotificationOverrides != nil && m.NotificationOverrides.Deploy.Disabled {
		return true
	}
	return false
}

// IsService returns whether the project is a service or not
func (m *Manifest) IsService() bool {
	return m.ImageType == ImageService
}

// IsTierOrHigher returns whether the tier is equal to or higher to the specified tier.
// Tier 0 is higher than 1
func (m *Manifest) IsTierOrHigher(tier int) bool {
	return m.ServiceTier != nil && *m.ServiceTier <= tier
}

// IsTier0 returns whether the service is tier 0 defaults to false if not set
func (m *Manifest) IsTier0() bool {
	return m.IsTierOrHigher(0)
}

var errDeployStageNotFound = errors.New("DeployStageNotFound")

// IsDeployStageNotFound determines whether the error is
// that of deployStageNotFound.
func IsDeployStageNotFound(err error) bool {
	return err == errDeployStageNotFound
}

// GetDeployStepIndex returns the step index of the jobName if it
// exists within the deploy steps and returns -1 and an error if it does not.
func (m *Manifest) GetDeployStepIndex(jobName string) (int, error) {
	for stepIndex, step := range m.DeploySteps {
		for _, job := range step {
			if job.Name == jobName {
				return stepIndex, nil
			}
		}
	}
	return -1, errDeployStageNotFound
}

// IsKubernetesOnly determines if a project is exclusively on
// Kubernetes. The assumption here is that it contains no legacy
// deployments, and has at least one Kubernetes enabled step.
func (m *Manifest) IsKubernetesOnly() bool {
	// if the kubernetes_only bool is set, that's all we care about.
	if m.KubernetesOnly != nil {
		return *m.KubernetesOnly
	}

	// otherwise:
	// conditions in which we are NOT kubernetes:
	// if we have any legacy: true deploy steps
	// if we have any legacydeployandreleasejob facets being deployed
	//
	// to be exclusively kubernetes, we must have at least one
	// kubernetes step.
	hasKubernetesStep := false
	for _, stage := range m.Deployments.List() {
		if stage.Legacy {
			return false
		}

		for _, target := range stage.Targets {
			if target.Facet.Type == FacetTypeLegacyDeployReleaseJob {
				return false
			}
		}

		if stage.Kubernetes.Enabled {
			hasKubernetesStep = true
		}
	}
	return hasKubernetesStep
}

/////////////////////////////////////////////

// AggregatedManifest is the collection of all manifests
type AggregatedManifest map[string]*Manifest

// ParseErrors is a collection of errors encountered when parsing
type ParseErrors map[string]error

// LoadAggregatedManifest attempts to parse the aggregated manifest from the given reader
func LoadAggregatedManifest(reader io.Reader) (AggregatedManifest, ParseErrors, error) {
	// We load into a generic interface first so we can use manifest.Load()
	var gen map[string]json.RawMessage
	dec := json.NewDecoder(reader)
	if err := dec.Decode(&gen); err != nil {
		return nil, nil, errors.Wrap(err, "aggrefest load failure")
	}

	a, pe := make(AggregatedManifest), make(ParseErrors)
	for k, b := range gen {
		var m Manifest
		if err := json.Unmarshal(b, &m); err != nil {
			pe[k] = err
			continue
		}
		a[k] = &m
	}
	if len(a) == 0 {
		return a, pe, errors.New("unable to load any manifest entries")
	}
	return a, pe, nil
}

/////////////////////////////////////////////

// ClusterAlias represents a manifest defined ClusterAlias
type ClusterAlias struct {
	Name string `json:"name,omitempty"`

	// ClusterLabels is a set of labels to match against. Keys are
	// the label key, value is a specific value to match against. An empty
	// value matches against all keys for that cluster
	ClusterLabels map[string][]string `mapstructure:"cluster_labels"`

	// ClusterTolerations is a set of taints specified on the cluster this facet will
	// tolerate.
	ClusterTolerations map[string][]string `mapstructure:"cluster_tolerations"`

	// When adding new exported fields please add `json:"-"` or `json:",omitempty"` to ensure the fields are exported correctly
}

// ClusterAliases is a collection of ClusterAlias objects referencable by name
type ClusterAliases struct {
	byName map[string]ClusterAlias
}

// Add a clusteralias to the collection
func (cas *ClusterAliases) Add(ca ClusterAlias) error {
	if cas.byName == nil {
		cas.Init()
	}
	if _, ok := cas.byName[ca.Name]; ok {
		if _, ok := preDefinedClusterAliases[ca.Name]; ok { // Don't allow users to override predefined cluster aliases
			return fmt.Errorf("%v is a reserved name for a predefined ClusterAlias", ca.Name)
		}
		return fmt.Errorf("ClusterAlias with Name %v already present", ca.Name)
	}
	cas.byName[ca.Name] = ca
	return nil
}

// Init initializes cluster aliases from preDefinedClusterAliases values
func (cas *ClusterAliases) Init() {
	cas.byName = make(map[string]ClusterAlias)
	for caName, caValue := range preDefinedClusterAliases {
		cas.byName[caName] = caValue
	}
}
