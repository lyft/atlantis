package manifest

type protoGroup struct {
	Name    string
	Members []string
}

type resolver struct {
	prototypes []protoGroup
	localName  string
	containers Containers
	groups     *Groups
}

// resolveGroups is a DFS over all Members and their groups, but ignoring
// remote groups (which are carted off to a separate list in Group). By resolving
// a group, each group actually contains the final list of all containers in that group
// and loses information on which prior groups got there.
// As traversal occurs, the individual groups memoize their resolution status
// to avoid having to re-resolve these groups at a later time (even though any real
// performance advantage here is near zero)
func (r *resolver) resolveGroups(
	targetGroup *Group,
	visited map[string]bool,
	prototype *protoGroup) (*Group, error) {

	// Ensure a group exists
	realGroup := r.groups.GetOrNew(prototype.Name)

	if realGroup.resolved {
		return realGroup, nil
	}

	if _, ok := visited[prototype.Name]; ok {
		return realGroup, nil
	}
	visited[prototype.Name] = true

	resolvedNames := map[string]bool{}

	for _, member := range prototype.Members {

		// We ignore remote Members and just stash them away - they aren't useful
		// for deployments at this time.
		lMember, err := localMember(member)
		if err != nil {
			return nil, err
		}
		if !isLocalMember(r.localName, member) {
			resolvedNames[member] = true
			realGroup.RemoteMembers = append(realGroup.RemoteMembers, member)
			targetGroup.RemoteMembers = append(targetGroup.RemoteMembers, member)
			continue
		}

		// Match a direct container object
		c, err := r.containers.GetByName(lMember)
		if err == nil {
			resolvedNames[member] = true
			realGroup.Containers[member] = c
			targetGroup.Containers[member] = c
			continue
		}

		// Match a group that is already pre-resolved
		if g, err := r.groups.GetByName(lMember); err == nil {
			if g.resolved {
				for gn, gc := range g.Containers {
					realGroup.Containers[gn] = gc
					targetGroup.Containers[gn] = gc
				}
				resolvedNames[member] = true
				continue
			}
		}

		// If no resolved groups exist, we still need to resolve a prototype. Lets go down
		// that path and try to locate one. If the group is already resolved due to this traversal
		// we can then consider this phase resolved
		for _, next := range r.prototypes {
			local, err := localMember(member)
			if err != nil {
				return nil, err
			}
			if next.Name == local {
				group, err := r.resolveGroups(targetGroup, visited, &next)
				if err != nil {
					return nil, err
				}
				if group.resolved {
					for gn, gc := range group.Containers {
						targetGroup.Containers[gn] = gc
						realGroup.Containers[gn] = gc
					}
					resolvedNames[member] = true
				}
			}
		}
	}

	prototype.Members = removeStringsLookup(prototype.Members, resolvedNames)
	// If the incoming set is 0, we are now done
	if len(prototype.Members) == 0 {
		realGroup.resolved = true
	}

	return realGroup, nil
}
