package service

import (
	"testing"
	"time"

	"upstream-x/internal/models"
	"upstream-x/internal/utils"
)

func TestBuildScreenGroupSummariesAggregatesObservedUsage(t *testing.T) {
	t.Parallel()

	groups := []models.GroupSnapshot{
		{GroupKey: "created", GroupName: "Created", IsUsedByToken: true, TokenUsageCount: 2},
		{GroupKey: "active", GroupName: "Active", IsUsedByToken: true, TokenUsageCount: 2},
		{GroupKey: "last-used", GroupName: "Last Used", IsUsedByToken: true, TokenUsageCount: 1},
		{GroupKey: "uncollected", GroupName: "Uncollected", IsUsedByToken: true, TokenUsageCount: 1},
		{GroupKey: "unbound", GroupName: "Unbound"},
	}
	lastUsedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	tokens := []models.TokenSnapshot{
		{GroupKey: "created"},
		{GroupKey: "created", UsedQuotaUSD: utils.PtrFloat64(0)},
		{GroupKey: "active", UsedQuotaUSD: utils.PtrFloat64(1.25)},
		{GroupKey: "active", UsedQuotaUSD: utils.PtrFloat64(0.75)},
		{GroupKey: "last-used", LastUsedAt: &lastUsedAt},
		{GroupKey: "uncollected"},
	}

	summaries := buildScreenGroupSummaries(groups, tokens)
	if len(summaries) != 4 {
		t.Fatalf("summaries length = %d, want 4", len(summaries))
	}
	if summaries[0].GroupKey != "active" || !summaries[0].HasObservedUsage {
		t.Fatalf("first summary = %#v, want active usage first", summaries[0])
	}
	if summaries[0].UsedQuotaUSD == nil || *summaries[0].UsedQuotaUSD != 2 {
		t.Fatalf("active used quota = %v, want 2", summaries[0].UsedQuotaUSD)
	}
	if summaries[1].GroupKey != "last-used" || !summaries[1].HasObservedUsage || summaries[1].UsedQuotaUSD != nil {
		t.Fatalf("last-used summary = %#v", summaries[1])
	}
	if summaries[2].GroupKey != "created" || summaries[2].UsedQuotaUSD == nil || *summaries[2].UsedQuotaUSD != 0 || summaries[2].HasObservedUsage {
		t.Fatalf("created summary = %#v", summaries[2])
	}
	if summaries[3].GroupKey != "uncollected" || summaries[3].UsedQuotaUSD != nil || summaries[3].HasObservedUsage {
		t.Fatalf("uncollected summary = %#v", summaries[3])
	}
}
