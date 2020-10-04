package manifest

func removeStringsLookup(s []string, r map[string]bool) []string {
	sr := s[:0]
	for _, v := range s {
		if _, ok := r[v]; !ok {
			sr = append(sr, v)
		}
	}
	return sr
}
