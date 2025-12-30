package filedb

import (
	"sort"
	"strings"
)

type StringSortable interface {
	SortString() string
}

type StringSorter[T StringSortable] struct {
	items   []T
	strings []string
}

func SortStringSortables[T StringSortable](items []T) []T {
	sorter := StringSorter[T]{
		items:   items,
		strings: make([]string, len(items)),
	}
	for i, item := range items {
		sorter.strings[i] = item.SortString()
	}
	sort.Sort(sorter)
	return sorter.items
}

func (l StringSorter[T]) Len() int           { return len(l.items) }
func (l StringSorter[T]) Less(i, j int) bool { return strings.Compare(l.strings[i], l.strings[j]) < 0 }
func (l StringSorter[T]) Swap(i, j int) {
	l.items[i], l.items[j] = l.items[j], l.items[i]
	l.strings[i], l.strings[j] = l.strings[j], l.strings[i]
}
