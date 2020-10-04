package manifest

import (
	"fmt"

	"github.com/pkg/errors"
)

type protoFacetGroup struct {
	Name    string
	Members []string
}

type facetGroupResolver struct {
	prototypes []protoFacetGroup
	facets     Facets
	groups     *FacetGroups
}

func populateFacetGroups(facetGroupList interface{}, facets Facets, facetGroups *FacetGroups) error {
	list, ok := facetGroupList.([]interface{})
	if !ok {
		return errors.Errorf("facetGroups neeeds to be a list")
	}

	prototypes := []protoFacetGroup{}
	for _, input := range list {
		var prototype protoFacetGroup
		decoder, err := newDecoder(&prototype)
		if err != nil {
			return errors.Wrap(err, "Can't build mapstructure decoder")
		}

		err = decoder.Decode(input)
		if err != nil {
			return errors.Wrapf(err, "Can't decode facet group prototype %v", input)
		}
		if _, err := facets.GetByName(prototype.Name); err == nil {
			return fmt.Errorf("facet group %v has the same name as a facet", prototype.Name)
		}
		prototypes = append(prototypes, prototype)
	}

	unresolved := []string{}
	res := facetGroupResolver{
		prototypes: prototypes,
		facets:     facets,
		groups:     facetGroups,
	}
	facetGroupNames := map[string]bool{}
	for _, prototype := range prototypes {
		if facetGroupNames[prototype.Name] {
			return errors.New(fmt.Sprintf("duplicate facet group names found: %s", prototype.Name))
		}
		facetGroupNames[prototype.Name] = true
		visited := map[string]bool{}
		// This is this target facet group where all results are merged into
		realFacetGroup := res.groups.GetOrNew(prototype.Name)
		g, err := res.resolveFacetGroups(realFacetGroup, visited, &prototype)
		if err != nil {
			return err
		}
		if !g.resolved {
			unresolved = append(unresolved, g.Name)
		}
	}
	if len(unresolved) > 0 {
		return errors.New(fmt.Sprintf("not all facet groups can be resolved: %v", unresolved))
	}
	return nil
}

func (r *facetGroupResolver) resolveFacetGroups(realFacetGroup *FacetGroup, visited map[string]bool, prototype *protoFacetGroup) (*FacetGroup, error) {
	if realFacetGroup.resolved {
		return realFacetGroup, nil
	}

	if _, ok := visited[prototype.Name]; ok {
		return realFacetGroup, nil
	}
	visited[prototype.Name] = true

	resolvedNames := map[string]bool{}

	for _, member := range prototype.Members {
		// Match a direct container object
		facet, err := r.facets.GetByName(member)
		if err == nil {
			resolvedNames[member] = true
			realFacetGroup.Members[member] = facet
			continue
		}

		// Match a group that is already pre-resolved
		if group, err := r.groups.GetByName(member); err == nil {
			if group.resolved {
				for name, facet := range group.Members {
					realFacetGroup.Members[name] = facet
				}
				resolvedNames[member] = true
				continue
			}
		}

		for _, next := range r.prototypes {
			if next.Name == member {
				group, err := r.resolveFacetGroups(realFacetGroup, visited, &next)
				if err != nil {
					return nil, err
				}
				if group.resolved {
					for name, facet := range group.Members {
						realFacetGroup.Members[name] = facet
					}
					resolvedNames[member] = true
				}
			}
		}
	}

	prototype.Members = removeStringsLookup(prototype.Members, resolvedNames)
	if len(prototype.Members) == 0 {
		realFacetGroup.resolved = true
	}
	return realFacetGroup, nil
}
