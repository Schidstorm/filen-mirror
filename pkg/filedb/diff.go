package filedb

import (
	"sort"
)

func toDiffSlice(ft *FileTree) []*fileTreeNodeInternal {
	result := make([]*fileTreeNodeInternal, 0, len(ft.nodes))
	for _, node := range ft.nodes {
		result = append(result, node)
	}
	return result
}

type nodeList []*fileTreeNodeInternal

func (l nodeList) Len() int           { return len(l) }
func (l nodeList) Less(i, j int) bool { return CompareUuids(&l[i].Uuid, &l[j].Uuid) < 0 }
func (l nodeList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func Diff(is, should *FileTree) chan DiffItem {
	diffChannel := make(chan DiffItem, 100)
	go diff(is, should, diffChannel)

	return diffChannel
}

func diff(is, should *FileTree, diffChannel chan DiffItem) {
	defer close(diffChannel)

	isNodes := nodeList(toDiffSlice(is))
	sort.Sort(isNodes)
	shouldNodes := nodeList(toDiffSlice(should))
	sort.Sort(shouldNodes)

	i, j := 0, 0
	for i < len(isNodes) && j < len(shouldNodes) {
		isNode := isNodes[i]
		shouldNode := shouldNodes[j]

		if isNode.Uuid == shouldNode.Uuid {
			var unequal bool
			if !isNode.Modtime.Equal(shouldNode.Modtime) {
				unequal = true
			} else if isNode.getParentUuid() != shouldNode.getParentUuid() {
				unequal = true
			} else if isNode.IsDir != shouldNode.IsDir {
				unequal = true
			} else if isNode.Hash != shouldNode.Hash {
				unequal = true
			}

			if unequal {
				oldPath := isNode.GetPath()
				newPath := shouldNode.GetPath()
				diffChannel <- DiffModified{
					Uuid:    isNode.Uuid,
					OldPath: oldPath,
					NewPath: newPath,
				}
			}
			i++
			j++
		} else if CompareUuids(&isNode.Uuid, &shouldNode.Uuid) < 0 {
			oldPath := isNode.GetPath()
			diffChannel <- DiffRemoved{
				Uuid: isNode.Uuid,
				Path: oldPath,
			}
			i++
		} else {
			newPath := shouldNode.GetPath()
			diffChannel <- DiffAdded{
				Uuid: shouldNode.Uuid,
				Path: newPath,
			}
			j++
		}
	}

	for i < len(isNodes) {
		oldPath := isNodes[i].GetPath()
		diffChannel <- DiffRemoved{
			Uuid: isNodes[i].Uuid,
			Path: oldPath,
		}
		i++
	}

	for j < len(shouldNodes) {
		newPath := shouldNodes[j].GetPath()
		diffChannel <- DiffAdded{
			Uuid: shouldNodes[j].Uuid,
			Path: newPath,
		}

		j++
	}
}
