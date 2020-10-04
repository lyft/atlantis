package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

var validSubDNS = regexp.MustCompile(`^([0-9a-z\-]){1,63}$`)
var validFacetName = regexp.MustCompile(`^([0-9a-z]){1,52}$`)
var validWatchedFile = regexp.MustCompile(`^([0-9a-z\-_]){1,63}$`)

func isLocalMember(localName string, member string) bool {
	parts := strings.Split(member, ".")
	return parts[0] == localName
}

func localMember(member string) (string, error) {
	parts := strings.Split(member, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("member not in [project].[member] syntax: %s", member)
	}
	return parts[1], nil
}

func isValidSubName(name string) bool {
	return validSubDNS.MatchString(name)
}

func isValidFacetName(name string) bool {
	return validFacetName.MatchString(name)
}

func isValidFileName(name string) bool {
	return validWatchedFile.MatchString(name)
}
