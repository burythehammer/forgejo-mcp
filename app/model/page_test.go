// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"encoding/json"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
)

func TestPageMetaOfCopiesTheListPage(t *testing.T) {
	total := 9
	got := PageMetaOf(list.Page[int]{
		Items: []int{1, 2}, Page: 2, Limit: 5, Count: 2, HasNext: true, Total: &total,
	})
	want := PageMeta{Page: 2, Limit: 5, Count: 2, HasNext: true, TotalCount: &total}
	if got.Page != want.Page || got.Limit != want.Limit || got.Count != want.Count ||
		got.HasNext != want.HasNext || got.TotalCount == nil || *got.TotalCount != 9 {
		t.Fatalf("PageMetaOf = %+v, want %+v", got, want)
	}
}

func TestPageMetaOfUnknownTotalIsNil(t *testing.T) {
	got := PageMetaOf(list.Page[int]{Page: 1, Limit: 3, Count: 0})
	if got.TotalCount != nil {
		t.Fatalf("unknown total = %v, want nil", *got.TotalCount)
	}
}

// The embedded meta keeps the wire order of the hand-written fields it
// replaces: the type's own fields first, then page, limit, count, has_next and
// total_count, with total_count omitted when nil.
func TestEmbeddedPageMetaKeepsWireOrder(t *testing.T) {
	total := 7
	withTotal, err := json.Marshal(struct {
		Items []int `json:"items"`
		PageMeta
	}{Items: []int{1}, PageMeta: PageMeta{Page: 1, Limit: 30, Count: 1, HasNext: false, TotalCount: &total}})
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
	}{Items: []int{1}, PageMeta: PageMeta{Page: 1, Limit: 30, Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	const wantWithoutTotal = `{"items":[1],"page":1,"limit":30,"count":1,"has_next":false}`
	if string(withoutTotal) != wantWithoutTotal {
		t.Fatalf("without total:\n got %s\nwant %s", withoutTotal, wantWithoutTotal)
	}
}
