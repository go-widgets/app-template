// SPDX-License-Identifier: BSD-3-Clause
//
// The ViewModel layer. This file holds ALL application state as
// go-widgets/mvvm primitives — Observables, an ObservableList and a
// Command — and NEVER references a widget. It is the single source of
// truth: the View (scene.go) binds widgets to these through
// go-widgets/mvvmtk, so state only ever changes here.
//
// Because the VM imports no widget package, it is trivially unit-testable
// on a native host (see viewmodel_test.go) with no painter or canvas.

package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-widgets/mvvm"
)

// item is one row of the catalog the template displays: a name and the
// category it belongs to. A real app swaps this for its own domain type.
type item struct {
	Name     string
	Category string
}

// sampleItems is the built-in demo dataset. A real app replaces this with
// its own data source (a fetch, an embed, a store). Kept small and neutral
// so the template is a self-contained, standalone page.
func sampleItems() []item {
	return []item{
		{"toolkit", "widgets"},
		{"tui", "widgets"},
		{"tray", "widgets"},
		{"mvvm", "state"},
		{"mvvmtk", "state"},
		{"mvvmlint", "state"},
		{"painter", "render"},
		{"svg", "render"},
		{"raster", "render"},
		{"canvas", "render"},
		{"vector", "render"},
		{"blend", "render"},
		{"sprite", "render"},
		{"atlas", "render"},
		{"opentype", "text"},
		{"shape", "text"},
		{"bidi", "text"},
		{"fonts", "text"},
		{"unicode", "text"},
		{"glyph", "text"},
		{"harfbuzz", "text"},
		{"layout", "text"},
		{"kerning", "text"},
		{"script", "text"},
	}
}

// viewModel is the whole application state. Every field is an mvvm
// primitive; the View binds widgets to them and the VM mutates only these.
type viewModel struct {
	all        []item   // the full, unfiltered catalog
	categories []string // ["All", ...distinct sorted categories]

	// Query is the case-insensitive name-substring filter (bound to a
	// SearchEntry). Category is the selected index into categories (bound to
	// a DropDown; 0 = "All"). Visible is the derived list of rows that pass
	// both filters (bound to a ListBox). Status is a human-readable count
	// (bound to a Label). Clear resets both filters (bound to a Button).
	Query    *mvvm.Observable[string]
	Category *mvvm.Observable[int]
	Visible  *mvvm.ObservableList[item]
	Status   *mvvm.Observable[string]
	Clear    *mvvm.Command
}

// newViewModel builds the VM from a dataset: it derives the category
// domain, wires the derived Visible list + Status to recompute on any
// filter change, and defines the Clear command (enabled only while a
// filter is active). recompute runs once so Visible/Status start correct.
func newViewModel(items []item) *viewModel {
	vm := &viewModel{
		all:        items,
		categories: append([]string{"All"}, distinctCategories(items)...),
		Query:      mvvm.NewObservable(""),
		Category:   mvvm.NewObservable(0),
		Visible:    mvvm.NewObservableList[item](),
		Status:     mvvm.NewObservable(""),
	}

	// Clear empties both filters; it can only execute while at least one
	// filter is active (so a bound button greys out when there is nothing to
	// clear).
	vm.Clear = mvvm.NewCommand(
		func() { vm.Query.Set(""); vm.Category.Set(0) },
		func() bool { return vm.Query.Get() != "" || vm.Category.Get() != 0 },
	)

	// Any filter change re-derives the visible list + status, and re-greys
	// the Clear command.
	vm.Query.SubscribeChanged(vm.recompute)
	vm.Category.SubscribeChanged(vm.recompute)
	mvvm.BindCanExecute(vm.Clear, vm.Query, vm.Category)

	vm.recompute()
	return vm
}

// distinctCategories returns the sorted unique Category values in items.
func distinctCategories(items []item) []string {
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		if !seen[it.Category] {
			seen[it.Category] = true
			out = append(out, it.Category)
		}
	}
	sort.Strings(out)
	return out
}

// selectedCategory returns the active category filter, or "" ("no
// constraint") when the DropDown sits on "All" (index 0) or out of range.
func (vm *viewModel) selectedCategory() string {
	i := vm.Category.Get()
	if i <= 0 || i >= len(vm.categories) {
		return ""
	}
	return vm.categories[i]
}

// recompute re-derives Visible (the rows passing the name + category
// filters, combined with AND) and Status (an "N of M shown" count). It is
// the ONLY writer of Visible/Status and touches no widget — the bound View
// updates itself. Called on construction and on every filter change.
func (vm *viewModel) recompute() {
	q := strings.ToLower(strings.TrimSpace(vm.Query.Get()))
	cat := vm.selectedCategory()

	var out []item
	for _, it := range vm.all {
		if q != "" && !strings.Contains(strings.ToLower(it.Name), q) {
			continue
		}
		if cat != "" && it.Category != cat {
			continue
		}
		out = append(out, it)
	}

	vm.Visible.Clear()
	vm.Visible.Append(out...)
	vm.Status.Set(strconv.Itoa(len(out)) + " of " + strconv.Itoa(len(vm.all)) + " shown")
}
