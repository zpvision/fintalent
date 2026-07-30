package matching

import (
	"sort"

	"FinTalent/internal/vacancymodule/domain"
)

// Calculate gives every participating block equal influence on the final score.
// Importance affects matching only inside its block; required is also checked globally.
func Calculate(requirements []domain.Requirement, resumeCategoryIDs []int64) domain.MatchResult {
	unique := make(map[[2]int64]domain.Requirement, len(requirements))
	for _, requirement := range requirements {
		key := [2]int64{requirement.BlockID, requirement.CategoryID}
		if _, exists := unique[key]; !exists {
			unique[key] = requirement
		}
	}
	if len(unique) == 0 {
		return domain.MatchResult{MandatoryMatch: true, Score: nil, MissingRequiredCategoryIDs: []int64{}}
	}
	resume := make(map[int64]struct{}, len(resumeCategoryIDs))
	for _, id := range resumeCategoryIDs {
		resume[id] = struct{}{}
	}
	type blockScore struct{ matched, total int }
	blocks := map[int64]blockScore{}
	missing := []int64{}
	for _, requirement := range unique {
		weight := domain.ImportanceCoefficients[requirement.Importance]
		block := blocks[requirement.BlockID]
		block.total += weight
		_, exists := resume[requirement.CategoryID]
		if exists {
			block.matched += weight
		} else if requirement.Importance == domain.ImportanceRequired {
			missing = append(missing, requirement.CategoryID)
		}
		blocks[requirement.BlockID] = block
	}
	totalScore := 0.0
	for _, block := range blocks {
		if block.total > 0 {
			totalScore += float64(block.matched) / float64(block.total) * 100
		}
	}
	value := totalScore / float64(len(blocks))
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return domain.MatchResult{MandatoryMatch: len(missing) == 0, MissingRequiredCategoryIDs: missing, Score: &value}
}
