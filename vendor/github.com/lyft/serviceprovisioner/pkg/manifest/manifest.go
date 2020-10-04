package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/lyft/serviceprovisioner/pkg/util"
)

type Validator interface {
	Validate() error
}

// Minimal service manifest for use in this package, only the "provisioning"
// field is parsed and the "mapstructure" tags are "-".
type Manifest struct {
	Name         string       `yaml:"name" json:"name" mapstructure:"-"`
	Provisioning Provisioning `yaml:"provisioning,omitempty" json:"provisioning,omitempty" mapstructure:"-"`
}

func (m *Manifest) Validate() error {
	return m.Provisioning.Validate()
}

// Top-level "provisioning" section of a manifest.
type Provisioning struct {
	S3Download  S3Download  `yaml:"s3_download,omitempty" json:"s3_download,omitempty" mapstructure:"s3_download"`
	DSTerraform DSTerraform `yaml:"dsterraform,omitempty" json:"dsterraform,omitempty" mapstructure:"dsterraform"`
	Script      Script      `yaml:"script,omitempty" json:"script,omitempty" mapstructure:"script"`
}

func (p *Provisioning) IsEmpty() bool {
	return !p.S3Download.HasSpecs() && !p.Script.IsEmpty()
}

func (p *Provisioning) Validate() error {
	if err := p.S3Download.Validate(); err != nil {
		return err
	}
	if err := p.Script.Validate(); err != nil {
		return err
	}
	return p.DSTerraform.Validate()
}

// DSTerraform is used to enable DSTerraform provisioning.  It is not used
// directly by this package, but is used by `datum` for validation.
type DSTerraform struct {
	// Optional list of containers to provision with DSTerraform, if omitted
	// all containers are provisioned.
	Containers []string `yaml:"containers,omitempty" json:"containers,omitempty" mapstructure:"containers"`
}

// Validate is a no-op as we don't validate the DSTerraform struct.
func (d *DSTerraform) Validate() error { return nil }

// Environment specific S3 settings.
//
// 	provisioning:
// 	  s3_download:
// 	    development:
// 	    ...
// 	    staging
// 	    ...
//
type S3Download struct {
	Development []DownloadSpec `yaml:"development,omitempty" json:"development,omitempty" mapstructure:"development"`
	Staging     []DownloadSpec `yaml:"staging,omitempty" json:"staging,omitempty" mapstructure:"staging"`
	Production  []DownloadSpec `yaml:"production,omitempty" json:"production,omitempty" mapstructure:"production"`
}

// HasSpecs returns if the S3Download has any specs.
func (s *S3Download) HasSpecs() bool {
	return s != nil && (len(s.Development) != 0 || len(s.Staging) != 0 ||
		len(s.Production) != 0)
}

func (s *S3Download) validateSpecs(environment string, specs []DownloadSpec) error {
	var errs []error
	for _, d := range specs {
		if err := d.Validate(); err != nil {
			if e, ok := err.(*util.MultiError); ok {
				errs = append(errs, e.Errors...)
			} else {
				errs = append(errs, err) // this should not happen
			}
		}
	}
	if len(errs) != 0 {
		return util.NewMultiError(
			"manifest",
			fmt.Sprintf("validating %q DownloadSpec", environment),
			errs,
		)
	}
	return nil
}

func (s *S3Download) Validate() error {
	// TODO (CEV): return all errors
	if err := s.validateSpecs("Development", s.Development); err != nil {
		return err
	}
	if err := s.validateSpecs("Staging", s.Staging); err != nil {
		return err
	}
	if err := s.validateSpecs("Production", s.Production); err != nil {
		return err
	}
	return nil
}

// TODO: rename and likely move to a Config directory or something
type DownloadSpec struct {
	Name string `yaml:"name" json:"name" mapstructure:"name"`
	URI  string `yaml:"s3_url" json:"s3_url" mapstructure:"s3_url"`

	// TODO (CEV): document 'Filename' and 'Directory'

	// Exact filename to save the file as.  This allows for renaming files.
	// It is an error if the S3 URL does not point to a single file.
	Filename string `yaml:"filename" json:"filename,omitempty" mapstructure:"filename"`

	// Directory to download file(s) to.
	Directory string `yaml:"directory" json:"directory,omitempty" mapstructure:"directory"`

	// Include/Exclude filters match against the base name of S3 Objects.
	Exclude GlobSet `yaml:"exclude,omitempty" json:"exclude,omitempty" mapstructure:"exclude"`
	Include GlobSet `yaml:"include,omitempty" json:"include,omitempty" mapstructure:"include"`

	// Optional list of containers to run S3 provisioning for.
	Containers []string `yaml:"containers" json:"containers"`

	// Extract any compressed files or archives.
	Extract bool `yaml:"extract" json:"extract" mapstructure:"extract"`

	// TODO: investigate the point at which it makes sense to hash vs.
	// simply downloading the file concurrently on NFS.
	CheckHash bool `yaml:"check_hash" json:"check_hash" mapstructure:"check_hash"`
}

func (d *DownloadSpec) Validate() error {
	var errs []error
	if d.Name == "" {
		errs = append(errs, errors.New("missing required param: Name"))
	}
	if d.URI == "" {
		errs = append(errs, errors.New("missing required param: URI"))
	} else {
		s := path.Clean(strings.TrimPrefix(strings.TrimSpace(d.URI), "s3://"))
		if s == "." {
			errs = append(errs, fmt.Errorf("invalid param: URI: cannot parse bucket: %q", d.URI))
		}
	}
	switch {
	case d.Directory != "" && d.Filename != "":
		errs = append(errs, errors.New("invalid param: both Directory and Filename cannot be specified"))
	case d.Filename != "":
		if !filepath.IsAbs(d.Filename) {
			errs = append(errs, errors.New("invalid param: Filename must be an absolute path got: "+d.Filename))
		}
	case d.Directory != "":
		if !filepath.IsAbs(d.Directory) {
			errs = append(errs, errors.New("invalid param: Directory must be an absolute path got: "+d.Directory))
		}
	default:
		errs = append(errs, errors.New("invalid param: either Directory or Filename must be specified"))
	}
	if len(errs) != 0 {
		return util.NewMultiError("manifest", "validating DownloadSpec", errs)
	}
	return nil
}

// FilterSet returns the FilterSet used for filtering S3 Objects.
// Filtering requires taking both the Include and Exclude into account,
// which the FilterSet does, so use it instead of those fields directly.
func (d *DownloadSpec) FilterSet() FilterSet {
	return FilterSet{Include: d.Include, Exclude: d.Exclude}
}

// MatchesContainer returns if the DownloadSpec matches container name, if
// no containers are specified true is returned.
func (d *DownloadSpec) MatchesContainer(name string) bool {
	return containsStringOrEmpty(d.Containers, name)
}

// A Script defines the provisioning scripts to execute for a container.
// See the ProvisioningScript comments for a detailed explanation of how
// the script is executed.
//
// We match the format S3Download format of specifying per-instance (dev,
// staging, prod) scripts, but only Development is currently supported.
type Script struct {
	Development ProvisioningScript `yaml:"development,omitempty" json:"development,omitempty" mapstructure:"development"`
	// Staging     Not Supported
	// Production  Not Supported
}

func (s *Script) IsEmpty() bool { return s.Development.IsEmpty() }

func (s *Script) Validate() error {
	if s.IsEmpty() {
		return nil
	}
	return s.Development.Validate()
}

// A ProvisioningScript defines the actual script to execute.
type ProvisioningScript struct {
	// Name of the script, really just used for logging.
	Name string `yaml:"name" json:"name" mapstructure:"name"`

	// Script to execute.  The script will be exec'd and thus requires a
	// hashbang (#!).  Any non-zero exit code is considered a failure.
	// The script is executed from the root "/" directory and will *not*
	// have access the container therefore it cannot depend on source code
	// being available.
	//
	// The scripts STDOUT and STDERR are captured, but truncated to a 256Kb.
	// STDOUT is always logged and STDERR is logged when there is an error.
	Script string `yaml:"script" json:"script" mapstructure:"script"`

	// Containers is optional and specifies the containers a script should
	// be executed for.  If omitted, the script will run for all containers.
	Containers []string `yaml:"containers,omitempty" json:"containers,omitempty" mapstructure:"containers"`
}

func (p *ProvisioningScript) IsEmpty() bool { return p.Script == "" }

// minimal check for a hashbang
var hashbangRe = regexp.MustCompile(`^ *#! *(?:/[[:alnum:]]+)+`)
var scriptNameRe = regexp.MustCompile(`[^-_. [:alnum:]]`)

func (p *ProvisioningScript) Validate() error {
	var errs []error
	if p.Name == "" {
		errs = append(errs, errors.New("missing required param: Name"))
	}
	if s := strings.TrimSpace(p.Name); s != p.Name {
		errs = append(errs, errors.New("invalid param: Name: contain contain leading/trailing whitespace"))
	}
	if scriptNameRe.MatchString(p.Name) {
		errs = append(errs, fmt.Errorf("script name (%s) must not match regex: `%s`",
			p.Name, scriptNameRe.String()))
	}
	if p.Script == "" {
		errs = append(errs, errors.New("missing required param: Script"))
	}
	if !hashbangRe.MatchString(p.Script) {
		errs = append(errs, errors.New("missing hashbang (#!)"))
	}
	if len(errs) != 0 {
		return util.NewMultiError("manifest", "validating ProvisioningScript", errs)
	}
	return nil
}

// MatchesContainer returns if the ProvisioningScript matches container name, if
// no containers are specified true is returned.
func (p *ProvisioningScript) MatchesContainer(name string) bool {
	return containsStringOrEmpty(p.Containers, name)
}

// ParseConfigYAML reads and parses a YAML encoded Config from r.
func ParseManifestYAML(r io.Reader) (*Manifest, error) {
	var m Manifest
	if err := decodeYAML(r, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func ParseManifestJSON(r io.Reader) (*Manifest, error) {
	var m Manifest
	if err := decodeJSON(r, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func ParseManifestFromFile(name string) (*Manifest, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if strings.HasSuffix(name, ".json") {
		return ParseManifestJSON(f)
	}
	return ParseManifestYAML(f)
}

func decodeJSON(r io.Reader, v Validator) error {
	if err := json.NewDecoder(r).Decode(v); err != nil {
		return err
	}
	return v.Validate()
}

func decodeYAML(r io.Reader, v Validator) error {
	if err := yaml.NewDecoder(r).Decode(v); err != nil {
		return err
	}
	return v.Validate()
}

func containsStringOrEmpty(a []string, s string) bool {
	if len(a) == 0 || s == "" {
		return true
	}
	for i := range a {
		if a[i] == s {
			return true
		}
	}
	return false
}
