/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiv2models

import (
	"fmt"
	"reflect"
	"testing"
)

func TestIndexChangelogItemsStableAcrossOrder(t *testing.T) {
	architecture := ChangelogItem{
		Field:     "Component",
		Fieldtype: "jira",
		FieldId:   "components",
		ToValue:   "41908",
		ToString:  "Architecture",
	}
	vanguard := ChangelogItem{
		Field:     "Component",
		Fieldtype: "jira",
		FieldId:   "components",
		ToValue:   "41929",
		ToString:  "Vanguard",
	}
	status := ChangelogItem{
		Field:      "status",
		Fieldtype:  "jira",
		FieldId:    "status",
		FromValue:  "1",
		FromString: "To Do",
		ToValue:    "3",
		ToString:   "In Progress",
	}

	forward := IndexChangelogItems([]ChangelogItem{vanguard, architecture, status})
	reversed := IndexChangelogItems([]ChangelogItem{status, architecture, vanguard})

	if !reflect.DeepEqual(indexPairs(forward), indexPairs(reversed)) {
		t.Fatalf("item indexes must not depend on JSON array order\nforward=%v\nreversed=%v", indexPairs(forward), indexPairs(reversed))
	}

	got := map[string]uint64{}
	for _, indexed := range forward {
		got[indexed.Item.Field+"|"+indexed.Item.ToValue] = indexed.ItemIndex
	}
	if got["Component|41908"] != 0 {
		t.Errorf("Architecture should be Component item_index 0, got %d", got["Component|41908"])
	}
	if got["Component|41929"] != 1 {
		t.Errorf("Vanguard should be Component item_index 1, got %d", got["Component|41929"])
	}
	if got["status|3"] != 0 {
		t.Errorf("status should stay item_index 0 independent of Component, got %d", got["status|3"])
	}
}

func TestIndexChangelogItemsDoesNotMutateInput(t *testing.T) {
	items := []ChangelogItem{
		{Field: "Component", ToValue: "2"},
		{Field: "Component", ToValue: "1"},
	}
	original := append([]ChangelogItem(nil), items...)
	_ = IndexChangelogItems(items)
	if !reflect.DeepEqual(items, original) {
		t.Fatalf("IndexChangelogItems mutated the input slice: got %#v want %#v", items, original)
	}
}

func TestChangelogItemToToolLayerSetsItemIndex(t *testing.T) {
	item := ChangelogItem{Field: "Component", ToValue: "41908", ToString: "Architecture"}
	got := item.ToToolLayer(2, 48452931, 1)
	if got.ItemIndex != 1 {
		t.Errorf("ItemIndex=%d, want 1", got.ItemIndex)
	}
	if got.ConnectionId != 2 || got.ChangelogId != 48452931 || got.Field != "Component" {
		t.Errorf("unexpected tool-layer identity: %+v", got)
	}
}

func indexPairs(items []IndexedChangelogItem) [][3]string {
	pairs := make([][3]string, 0, len(items))
	for _, indexed := range items {
		pairs = append(pairs, [3]string{
			indexed.Item.Field,
			indexed.Item.ToValue,
			formatIndex(indexed.ItemIndex),
		})
	}
	return pairs
}

func formatIndex(i uint64) string {
	return fmt.Sprintf("%d", i)
}
