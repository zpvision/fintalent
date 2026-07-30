package matching

import (
	"math"
	"reflect"
	"testing"

	"FinTalent/internal/vacancymodule/domain"
)

func requirement(category, block int64, importance string) domain.Requirement {
	return domain.Requirement{CategoryID: category, BlockID: block, Importance: importance}
}
func assertScore(t *testing.T, result domain.MatchResult, expected float64) {
	t.Helper()
	if result.Score == nil || math.Abs(*result.Score-expected) > .01 {
		t.Fatalf("score: got %#v, want %.2f", result.Score, expected)
	}
}
func TestFullAndMandatoryMatch(t *testing.T) {
	result := Calculate([]domain.Requirement{requirement(1, 1, "required"), requirement(2, 1, "preferred"), requirement(3, 1, "bonus")}, []int64{1, 2, 3})
	assertScore(t, result, 100)
	if !result.MandatoryMatch {
		t.Fatal("mandatory requirements must match")
	}
}
func TestMissingRequiredIsReported(t *testing.T) {
	result := Calculate([]domain.Requirement{requirement(1, 1, "required"), requirement(2, 1, "preferred")}, []int64{2})
	if result.MandatoryMatch || !reflect.DeepEqual(result.MissingRequiredCategoryIDs, []int64{1}) {
		t.Fatalf("unexpected result: %#v", result)
	}
	assertScore(t, result, 37.5)
}
func TestImportanceOrdering(t *testing.T) {
	requirements := []domain.Requirement{requirement(1, 1, "required"), requirement(2, 1, "preferred"), requirement(3, 1, "bonus")}
	required, preferred, bonus := Calculate(requirements, []int64{1}), Calculate(requirements, []int64{2}), Calculate(requirements, []int64{3})
	if !(*required.Score > *preferred.Score && *preferred.Score > *bonus.Score) {
		t.Fatalf("importance order is wrong: %v %v %v", *required.Score, *preferred.Score, *bonus.Score)
	}
}
func TestEmptyVacancyHasNullScore(t *testing.T) {
	if result := Calculate(nil, []int64{1}); result.Score != nil {
		t.Fatalf("expected nil score: %#v", result)
	}
}
func TestOrderAndDuplicatesDoNotChangeScore(t *testing.T) {
	a := []domain.Requirement{requirement(1, 1, "required"), requirement(2, 2, "bonus")}
	b := []domain.Requirement{a[1], a[0], a[0]}
	first, second := Calculate(a, []int64{1}), Calculate(b, []int64{1})
	assertScore(t, first, 50)
	assertScore(t, second, 50)
}
func TestBlocksHaveEqualInfluence(t *testing.T) {
	requirements := []domain.Requirement{requirement(1, 1, "required"), requirement(2, 1, "required"), requirement(3, 1, "required"), requirement(4, 1, "required"), requirement(5, 2, "required")}
	// The large first block is 100%, the single-item second block is 0%: final is 50%.
	result := Calculate(requirements, []int64{1, 2, 3, 4})
	assertScore(t, result, 50)
}
func TestImportanceComesFromVacancyRequirement(t *testing.T) {
	bonus := Calculate([]domain.Requirement{requirement(1, 1, "bonus"), requirement(2, 1, "required")}, []int64{1})
	required := Calculate([]domain.Requirement{requirement(1, 1, "required"), requirement(2, 1, "required")}, []int64{1})
	if *bonus.Score >= *required.Score {
		t.Fatalf("stored requirement importance was ignored: %v %v", *bonus.Score, *required.Score)
	}
}
