package controller

type Sortable struct {
	Key   string
	Value interface{}
}

type SortableArray struct {
	Array []*Sortable
}

func (sortableArray *SortableArray) Len() int {
	return len(sortableArray.Array)
}

func (sortableArray *SortableArray) Swap(i, j int) {
	sortableArray.Array[i], sortableArray.Array[j] = sortableArray.Array[j], sortableArray.Array[i]
}

func (sortableArray *SortableArray) Less(i, j int) bool {
	return sortableArray.Array[i].Key < sortableArray.Array[j].Key
}
