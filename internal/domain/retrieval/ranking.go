package retrieval

import "sort"

func SortCandidates(candidates []Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Memory.UpdatedAt.After(candidates[j].Memory.UpdatedAt)
		}
		return candidates[i].Score > candidates[j].Score
	})
}
