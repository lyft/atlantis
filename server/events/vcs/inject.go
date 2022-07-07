package vcs

// Declare all package dependencies here

func NewPullMergeabilityChecker(commitStatusPrefix string) MergeabilityChecker {
	statusFilters := newValidStatusFilters(commitStatusPrefix)
	checksFilters := newValidChecksFilters(commitStatusPrefix)

	return &PullMergeabilityChecker{
		supplementalChecker: newSupplementalMergeabilityChecker(statusFilters, checksFilters),
	}
}

func newValidStatusFilters(commitStatusPrefix string) []ValidStatusFilter {
	titleMatcher := StatusTitleMatcher{TitlePrefix: commitStatusPrefix}
	applyStatusFilter := &ApplyStatusFilter{
		statusTitleMatcher: titleMatcher,
	}

	return []ValidStatusFilter{
		SuccessStateFilter, applyStatusFilter,
	}
}

func newValidChecksFilters(commitStatusPrefix string) []ValidChecksFilter {
	titleMatcher := StatusTitleMatcher{TitlePrefix: commitStatusPrefix}
	applyChecksFilter := &ApplyChecksFilter{
		statusTitleMatcher: titleMatcher,
	}
	return []ValidChecksFilter{
		SuccessConclusionFilter, applyChecksFilter,
	}
}

func newSupplementalMergeabilityChecker(
	statusFilters []ValidStatusFilter,
	checksFilters []ValidChecksFilter,
) MergeabilityChecker {
	return &SupplementalMergabilityChecker{
		statusFilter:  statusFilters,
		checksFilters: checksFilters,
	}
}
