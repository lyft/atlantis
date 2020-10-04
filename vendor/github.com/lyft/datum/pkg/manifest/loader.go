package manifest

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/ghodss/yaml"
	provisioning "github.com/lyft/serviceprovisioner/pkg/manifest"
	pkgerrors "github.com/pkg/errors"
)

// ErrInvalidManifest is returned when a manifest is considered
// invalid.
var ErrInvalidManifest = fmt.Errorf("invalid manifest")

// Load attempts to parse a manifest in YAML from the supplied reader.
//
// Interestingly, a *Manifest is always returned, even in the case of
// an error.
func Load(reader io.Reader) (*Manifest, error) {
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "manifest load failed")
	}

	// We load into a generic interface first, so we can sort out exactly what we got
	// procedurally.
	var gen interface{}
	err = yaml.Unmarshal(b, &gen)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}

	var m Manifest
	err = populate(&m, gen)
	m.RawManifest = b
	if err != nil {
		err = fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	return &m, err
}

func populate(m *Manifest, from interface{}) error {
	root, ok := from.(map[string]interface{})
	if !ok {
		return fmt.Errorf("root of manifest was not a dictionary")
	}

	rootDecoder, err := newDecoder(m)
	if err != nil {
		return pkgerrors.Wrap(err, "can't build a mapstructure decoder")
	}
	// NOTE: decode is used to load all of the root level keys, so we purposefully drop
	// the error emitted from sub keys
	_ = rootDecoder.Decode(root)

	builderComponent, ok := root["builder"]
	if ok {
		var builder Builder
		if builder, err = parseBuilder(builderComponent); err != nil {
			return pkgerrors.Wrap(err, "invalid builder component")
		}
		m.Builder = builder
	} else {
		// Per control, specifying no builder is equivalent to the following:
		// https://github.com/lyft/control/blob/8498334ceeb2b6d8f0dd0348c7a90139339dee24/docs/manifest.md#L582
		m.Builder = &SingleBuilder{
			Name: "docker_build",
			Params: Params{
				Dockerfile: "Dockerfile",
				SubImage:   "",
			},
		}
	}

	containerList, ok := root["containers"]
	if ok {
		var containers Containers
		err = populateContainers(containerList, &containers, m.Builder)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode container list")
		}
		m.Containers = containers
	}

	clusterAliasList, ok := root["cluster_aliases"]
	if ok {
		var clusterAliases ClusterAliases
		err = populateClusterAliases(clusterAliasList, &clusterAliases)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode clusterAlias list")
		}
		m.ClusterAliases = clusterAliases
	}

	groupList, ok := root["groups"]
	if ok {
		var groups Groups
		err = populateGroups(m.Name, groupList, m.Containers, &groups)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode group list")
		}
		m.Groups = groups
	}

	facetList, ok := root["facets"]
	if ok {
		var facets Facets
		var envoyServices []EnvoyService
		if m.Envoy != nil {
			envoyServices = m.Envoy.Settings.Services
		}
		err = populateFacets(facetList, m.Name, m.Groups, m.Containers, envoyServices, &facets)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode facets")
		}
		m.Facets = facets
	}

	facetGroupsList, ok := root["facet_groups"]
	if ok {
		var facetGroups FacetGroups
		err = populateFacetGroups(facetGroupsList, m.Facets, &facetGroups)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode facet groups")
		}
		m.FacetGroups = facetGroups
	}

	deployList, ok := root["deploy"]
	if ok {
		var deploys DeploymentStages
		if m.ClusterAliases.byName == nil {
			m.ClusterAliases.Init()
		}
		root, err := parseDeploymentNode(deployList, m)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to parse deployment list")
		}
		err = populateDeploys(root, &deploys)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode deploys list")
		}
		m.Deployments = deploys
		m.DeploySteps = root.GetStagesInSteps()
	}

	rawSettings, ok := root["deploy_settings"]
	m.DeploySettings = DeploySettings{
		MaxCommits: defaultMaxCommits,
	}
	if ok {
		var deploySettings DeploySettings
		if err := populateDeploySettings(rawSettings, &deploySettings); err != nil {
			return pkgerrors.Wrap(err, "invalid deploy settings")
		}
		m.DeploySettings = deploySettings
	}

	var dataDeploy DataDeploy
	err = populateDataDeployComponents(root, &dataDeploy)
	if err != nil {
		return pkgerrors.Wrap(err, "Can't decode data deploy components")
	}
	m.Data = dataDeploy

	if vv, ok := root["provisioning"]; ok {
		var p provisioning.Provisioning
		if err := populateProvisioning(vv, &p); err != nil {
			return pkgerrors.Wrap(err, "invalid provisioning component")
		}
		m.Provisioning = p
	}

	if orchestrationRoot, ok := root["orchestration"]; ok {
		err := populateOrchestration(orchestrationRoot, m)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't decode orchestration components")
		}
	}

	return nil
}

func populateGroups(localName string, groupList interface{}, containers Containers, groups *Groups) error {
	list, ok := groupList.([]interface{})
	if !ok {
		return pkgerrors.New("input was not a list to group parsing")
	}

	prototypes := []protoGroup{}
	containerGroupNames := make(map[string]bool)
	for _, groupInput := range list {
		var prototype protoGroup
		decoder, err := newDecoder(&prototype)
		if err != nil {
			return pkgerrors.Wrap(err, "Can't build mapstructure decoder")
		}

		err = decoder.Decode(groupInput)
		if err != nil {
			return pkgerrors.Wrapf(err, "Can't decode group prototype %v", groupInput)
		}
		if _, err := containers.GetByName(prototype.Name); err == nil {
			return fmt.Errorf("group would shadow a container - naming conflicts not allowed: group %v", prototype.Name)
		}
		if containerGroupNames[prototype.Name] {
			return fmt.Errorf("duplicate container group names found: %s", prototype.Name)
		}
		containerGroupNames[prototype.Name] = true

		prototypes = append(prototypes, prototype)
	}

	unresolved := []string{}
	for _, prototype := range prototypes {
		visited := make(map[string]bool)
		res := resolver{
			prototypes: prototypes,
			localName:  localName,
			containers: containers,
			groups:     groups,
		}
		// This is this target group where all results are merged into
		realGroup := res.groups.GetOrNew(prototype.Name)
		g, err := res.resolveGroups(realGroup, visited, &prototype)
		if err != nil {
			return err
		}
		if !g.resolved {
			unresolved = append(unresolved, g.Name)
		}
	}
	if len(unresolved) > 0 {
		return pkgerrors.New(fmt.Sprintf("not all groups can be resolved: %v", unresolved))
	}

	return nil
}

func populateContainers(containerList interface{}, containers *Containers, builder Builder) error {
	validSubImageNames := make(map[string]bool)
	for _, subImageName := range builder.SubImageNames() {
		validSubImageNames[subImageName] = true
	}

	if list, ok := containerList.([]interface{}); ok {
		for _, prototype := range list {

			var container Container
			decoder, err := newStrictDecoder(&container)
			if err != nil {
				return pkgerrors.Wrap(err, "Can't build mapstructure decoder")
			}

			err = decoder.Decode(prototype)
			if err != nil {
				return pkgerrors.Wrapf(err, "decoding structure %v failed", prototype)
			}

			if container.Name == "" {
				return pkgerrors.New("need to specify a container name")
			}

			if !isValidSubName(container.Name) {
				return pkgerrors.Errorf("container %s does not have a valid name ([a-z0-9.-])", container.Name)
			}

			if !validSubImageNames[container.SubImage] {
				return pkgerrors.Errorf("container %s references in invalid sub_image %s", container.Name, container.SubImage)
			}

			err = containers.Add(&container)
			if err != nil {
				return pkgerrors.Wrap(err, "unable to add container to list")
			}
		}
	} else {
		return pkgerrors.New("Did not get a list of containers")
	}
	return nil
}

func populateProvisioning(vv interface{}, prov *provisioning.Provisioning) error {
	m, ok := vv.(map[string]interface{})
	if !ok {
		return pkgerrors.New("Invalid provisioning block") // TODO: improve
	}
	dec, err := newStrictDecoder(prov)
	if err != nil {
		return pkgerrors.Wrap(err, "Can't build mapstructure decoder")
	}
	if err := dec.Decode(m); err != nil {
		return pkgerrors.Wrapf(err, "decoding Provisioning structure %v failed", m)
	}
	return prov.Validate()
}

func populateClusterAliases(clusterAliasList interface{}, clusterAliases *ClusterAliases) error {
	if list, ok := clusterAliasList.([]interface{}); ok {

		cluserAliasNames := make(map[string]bool)
		for _, prototype := range list {

			var clusterAlias ClusterAlias
			decoder, err := newStrictDecoder(&clusterAlias)
			if err != nil {
				return pkgerrors.Wrap(err, "Can't build mapstructure decoder")
			}

			err = decoder.Decode(prototype)
			if err != nil {
				return pkgerrors.Wrapf(err, "decoding structure %v failed", prototype)
			}

			if clusterAlias.Name == "" {
				return pkgerrors.New("need to specify a clusterAlias name")
			}

			if !isValidSubName(clusterAlias.Name) {
				return pkgerrors.Errorf("clusterAlias %s does not have a valid name ([a-z0-9.-])", clusterAlias.Name)
			}
			if cluserAliasNames[clusterAlias.Name] {
				return fmt.Errorf("duplicate cluster alias names found: %s", clusterAlias.Name)
			}
			cluserAliasNames[clusterAlias.Name] = true

			err = validateClusterTolerations(clusterAlias.ClusterTolerations)
			if err != nil {
				return pkgerrors.Wrap(err, "Invalid ClusterToleration")
			}

			err = clusterAliases.Add(clusterAlias)
			if err != nil {
				return pkgerrors.Wrapf(err, "unable to add clusterAlias %s to list", clusterAlias.Name)
			}
		}
	} else {
		return pkgerrors.New("No clusterAliases defined under the cluster_aliases top level key")
	}
	return nil
}
