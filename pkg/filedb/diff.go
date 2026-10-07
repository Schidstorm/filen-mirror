package filedb

import (
	"sort"
)

type nodeList []*fileTreeNodeInternal

func (l nodeList) Len() int           { return len(l) }
func (l nodeList) Less(i, j int) bool { return CompareUuids(&l[i].Uuid, &l[j].Uuid) < 0 }
func (l nodeList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func toDiffSlice(ft *FileTree) nodeList {
	result := make(nodeList, 0, len(ft.nodes))
	for _, node := range ft.nodes {
		result = append(result, node)
	}
	return result
}

func StartDiff(is, should *FileTree) chan DiffItem {
	diffChannel := make(chan DiffItem, 100)
	go diff(is, should, diffChannel)

	return diffChannel
}

func diff(is, should *FileTree, diffChannel chan DiffItem) {
	defer close(diffChannel)

	if len(is.nodes) == 0 {
		shouldNodes := toDiffSlice(should)
		sort.Sort(shouldNodes)
		for _, node := range shouldNodes {
			diffChannel <- DiffAdded{
				Uuid: node.Uuid,
				Path: node.GetPath(),
			}
		}
		return
	}

	if len(should.nodes) == 0 {
		isNodes := toDiffSlice(is)
		sort.Sort(isNodes)
		for _, node := range isNodes {
			diffChannel <- DiffRemoved{
				Uuid: node.Uuid,
				Path: node.GetPath(),
			}
		}
		return
	}

	var changes nodeList
	for uuid, isNode := range is.nodes {
		shouldNode, exists := should.nodes[uuid]
		if !exists {
			changes = append(changes, isNode)
			continue
		}

		if !isNode.Modtime.Equal(shouldNode.Modtime) ||
			isNode.getParentUuid() != shouldNode.getParentUuid() ||
			isNode.IsDir != shouldNode.IsDir ||
			isNode.Hash != shouldNode.Hash {
			changes = append(changes, isNode)
		}
	}

	for uuid, shouldNode := range should.nodes {
		if _, exists := is.nodes[uuid]; !exists {
			changes = append(changes, shouldNode)
		}
	}

	sort.Sort(changes)

	for _, node := range changes {
		isNode, existsInIs := is.nodes[node.Uuid]
		shouldNode, existsInShould := should.nodes[node.Uuid]
		switch {
		case !existsInIs:
			diffChannel <- DiffAdded{
				Uuid: node.Uuid,
				Path: shouldNode.GetPath(),
			}
		case !existsInShould:
			diffChannel <- DiffRemoved{
				Uuid: node.Uuid,
				Path: isNode.GetPath(),
			}
		default:
			diffChannel <- DiffModified{
				Uuid:    node.Uuid,
				OldPath: isNode.GetPath(),
				NewPath: shouldNode.GetPath(),
			}
		}
	}
}
