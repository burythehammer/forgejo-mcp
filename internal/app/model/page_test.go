// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
)

func TestPageMetaOfCopiesTheListPage(t *testing.T) {
	total := 9
	got := PageMetaOf(list.Bounded[int]{
		Items: []int{1, 2}, Page: 2, Limit: 5, Count: 2, HasNext: true, Total: &total,
	})
	if got.Page != 2 || got.Limit != 5 || got.Count != 2 || !got.HasNext {
		t.Fatalf("PageMetaOf window = %+v", got)
	}
	if got.TotalCount == nil || *got.TotalCount != 9 {
		t.Fatalf("PageMetaOf total = %v, want 9", got.TotalCount)
	}
}

func TestPageMetaOfUnknownTotalIsNil(t *testing.T) {
	got := PageMetaOf(list.Bounded[int]{Page: 1, Limit: 3, Count: 0})
	if got.TotalCount != nil {
		t.Fatalf("unknown total = %v, want nil", got.TotalCount)
	}
}

// The common order is the one most page types have: the type's own fields,
// then page, limit, count, has_next and total_count, with total_count omitted
// when nil.
func TestEmbeddedPageMetaKeepsCommonWireOrder(t *testing.T) {
	total := 7
	withTotal, err := json.Marshal(struct {
		Items []int `json:"items"`
		PageMeta
	}{Items: []int{1}, PageMeta: PageMeta{
		PageWindow: PageWindow{Page: 1, Limit: 30, Count: 1},
		PageTotal:  PageTotal{TotalCount: &total},
	}})
	if err != nil {
		t.Fatal(err)
	}
	const wantWithTotal = `{"items":[1],"page":1,"limit":30,"count":1,"has_next":false,"total_count":7}`
	if string(withTotal) != wantWithTotal {
		t.Fatalf("with total:\n got %s\nwant %s", withTotal, wantWithTotal)
	}

	withoutTotal, err := json.Marshal(struct {
		Items []int `json:"items"`
		PageMeta
	}{Items: []int{1}, PageMeta: PageMeta{PageWindow: PageWindow{Page: 1, Limit: 30, Count: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	const wantWithoutTotal = `{"items":[1],"page":1,"limit":30,"count":1,"has_next":false}`
	if string(withoutTotal) != wantWithoutTotal {
		t.Fatalf("without total:\n got %s\nwant %s", withoutTotal, wantWithoutTotal)
	}
}

// The total-first order is the one of the workflow run, job, artifact and hook
// lists: total_count comes before has_next, so those types embed the parts
// separately, in that order.
func TestEmbeddedPageParts_TotalFirstOrder(t *testing.T) {
	total := 3
	got, err := json.Marshal(struct {
		Items []int `json:"items"`
		PageWindow
		PageTotal
		PageNext
	}{Items: []int{1}, PageWindow: PageWindow{Page: 2, Limit: 1, Count: 1},
		PageTotal: PageTotal{TotalCount: &total}, PageNext: PageNext{HasNext: true}})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"items":[1],"page":2,"limit":1,"count":1,"total_count":3,"has_next":true}`
	if string(got) != want {
		t.Fatalf("total-first:\n got %s\nwant %s", got, want)
	}
}
