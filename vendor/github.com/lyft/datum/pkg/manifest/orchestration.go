package manifest

import (
	"github.com/pkg/errors"
)

type protoPolicySource struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type protoPolicy struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Source protoPolicySource `mapstructure:"source" json:"source"`
	Owners []string          `mapstructure:"owners" json:"owners,omitempty"`
}

type protoTerraformRoot struct {
	Name             string   `json:"name"`
	Directory        string   `json:"directory"`
	WhenModified     []string `mapstructure:"when_modified" json:"when_modified,omitempty"`
	TerraformVersion string   `mapstructure:"terraform_version" json:"terraform_version,omitempty"`
	Policies         []string `mapstructure:"policies" json:"policies,omitempty"`
}

func extractByKey(rawRoot interface{}, key string) (interface{}, bool) {
	raw, ok := rawRoot.(map[string]interface{})[key]

	return raw, ok
}

func populateOrchestration(rawRoot interface{}, m *Manifest) error {
	// Policies are optional key.
	policies, ok := extractByKey(rawRoot, "policies")
	if ok {
		err := populatePolicyChecks(policies, &m.Orchestration.Policies)
		if err != nil {
			return err
		}
	}

	// Roots are optional as well.
	// If the repository is a shareable policy set it doesn't require roots
	// declaration
	terraformRoots, rootsPresent := extractByKey(rawRoot, "roots")
	if rootsPresent {
		err := populateTerraformRoots(terraformRoots, &m.Orchestration.Roots, &m.Orchestration.Policies)
		if err != nil {
			return err
		}
	}

	return nil
}

func populatePolicyChecks(rawPolicyChecks interface{}, Policies *Policies) error {
	for _, policy := range rawPolicyChecks.([]interface{}) {
		var prototype protoPolicy
		decoder, err := newStrictDecoder(&prototype)
		if err != nil {
			return errors.Wrap(err, "failed to decode opa policy due to decoder issue")
		}

		if err := decoder.Decode(policy); err != nil {
			return errors.Wrap(err, "failed to decode valid opa policy")
		}

		owners := make([]*PolicyOwner, len(prototype.Owners))
		for index, owner := range prototype.Owners {

			policyOwner := &PolicyOwner{
				Type: OwnerTypeUserName,
				Name: owner,
			}
			owners[index] = policyOwner
		}

		switch prototype.Type {
		case string(PolicyTypeHardMandatory):
		case string(PolicyTypeSoftMandatory):
		default:
			return errors.Errorf("policy type is invalid for policy %v (type %v)", prototype.Name, prototype.Type)
		}

		switch prototype.Source.Type {
		case string(PolicySourceLocal):
		case string(PolicySourceGithub):
		default:
			return errors.Errorf("policy source type is invalid for policy %v (type %v)", prototype.Name, prototype.Source.Type)
		}

		if prototype.Source.Path == "" {
			return errors.Errorf("policy source is missing path for policy %v", prototype.Name)
		}

		if prototype.Name == "" {
			return errors.New("policy name is required")
		}

		policy := &Policy{
			Name: prototype.Name,
			Type: PolicyType(prototype.Type),
			Source: PolicySource{
				Type: PolicySourceType(prototype.Source.Type),
				Path: prototype.Source.Path,
			},
			Owners: owners,
		}
		if err := Policies.Add(policy); err != nil {
			return errors.Wrap(err, "Adding Opa Policy failed")
		}
	}

	return nil
}

func populateTerraformRoots(rawTerraformRoots interface{}, terraformRoots *TerraformRoots, Policies *Policies) error {
	for _, tfRoot := range rawTerraformRoots.([]interface{}) {
		var prototype protoTerraformRoot

		decoder, err := newStrictDecoder(&prototype)
		if err != nil {
			return errors.Wrap(err, "failed to decode terraform root proto")
		}

		if err := decoder.Decode(tfRoot); err != nil {
			return errors.Wrap(err, "failed to decode valid terraform root")
		}

		var policies []*Policy

		if len(prototype.Policies) == 0 {
			policies = Policies.List()
		} else {
			for _, policyName := range prototype.Policies {
				policy, err := Policies.GetByName(policyName)
				if err != nil {
					return errors.Wrap(err, "policy doesn't exist")
				}

				policies = append(policies, policy)
			}
		}

		if prototype.Name == "" {
			return errors.New("specify root name(i.e production|staging)")
		}

		if prototype.Directory == "" {
			return errors.New("specify path to the root directory")
		}

		terraformRoot := &TerraformRoot{
			Name:             prototype.Name,
			Directory:        prototype.Directory,
			WhenModified:     prototype.WhenModified,
			TerraformVersion: prototype.TerraformVersion,
			Policies:         policies,
		}

		if err := terraformRoots.Add(terraformRoot); err != nil {
			return errors.Wrap(err, "Adding terraform root failed")
		}
	}
	return nil
}
