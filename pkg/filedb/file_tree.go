package filedb

import (
	"maps"
	"time"
)

type FileTree struct {
	nodes map[Uuid]*fileTreeNodeInternal
}

func NewFileTree() *FileTree {
	return &FileTree{
		nodes: make(map[Uuid]*fileTreeNodeInternal),
	}
}

func (ft *FileTree) CopyFrom(other *FileTree) {
	ft.nodes = make(map[Uuid]*fileTreeNodeInternal)
	maps.Copy(ft.nodes, other.nodes)
}

func (ft *FileTree) EnsureItems(items []FileTreeNode) {
	for _, item := range items {
		parent := ft.ensureParent(item.Parent)
		node, exists := ft.nodes[item.Uuid]
		if !exists {
			node = &fileTreeNodeInternal{
				Uuid: item.Uuid,
			}
		} else {
			node.invalidatePathCache()
		}

		node.Name = item.Name
		node.Hash = item.Hash
		node.Modtime = item.Modtime
		node.IsDir = item.IsDir
		node.Parent = parent

		ft.nodes[item.Uuid] = node
		if parent != nil {
			parent.Children = append(parent.Children, node)
		}
	}
}

func (ft *FileTree) CreateFile(uuid Uuid, parentUuid Uuid, name string, modtime time.Time, h string) {
	ft.EnsureItems([]FileTreeNode{{
		Uuid:    uuid,
		Name:    FileNameFromString(name),
		Hash:    HashFromString(h),
		Modtime: modtime,
		IsDir:   false,
		Parent:  parentUuid,
	}})
}

func (ft *FileTree) CreateDir(uuid Uuid, parentUuid Uuid, name string) {
	ft.EnsureItems([]FileTreeNode{{
		Uuid:   uuid,
		Name:   FileNameFromString(name),
		IsDir:  true,
		Parent: parentUuid,
	}})
}

func (ft *FileTree) ensureParent(uuid Uuid) *fileTreeNodeInternal {
	if uuid == NilUuid {
		return nil
	}

	parent, exists := ft.nodes[uuid]
	if !exists || !parent.IsDir {
		ft.EnsureItems([]FileTreeNode{{
			Uuid:   uuid,
			Name:   FileNameFromString(""),
			IsDir:  true,
			Parent: NilUuid,
		}})
		parent = ft.nodes[uuid]
	}
	return parent
}

func (ft *FileTree) Remove(uuid Uuid) {
	node, exists := ft.nodes[Uuid(uuid)]
	if !exists {
		return
	}
	delete(ft.nodes, Uuid(uuid))

	// Recursively remove children
	for _, child := range node.Children {
		ft.Remove(child.Uuid)
	}

}

func (ft *FileTree) Move(uuid Uuid, newParentUuid Uuid, newName FileName) {
	node, exists := ft.nodes[Uuid(uuid)]
	if !exists {
		return
	}
	node.invalidatePathCache()

	if node.Parent != nil {
		for i, child := range node.Parent.Children {
			if child.Uuid == uuid {
				// Remove from old parent's children
				node.Parent.Children = append(node.Parent.Children[:i], node.Parent.Children[i+1:]...)
				break
			}
		}
	}

	node.Parent = ft.ensureParent(newParentUuid)
	if node.Parent != nil {
		node.Parent.Children = append(node.Parent.Children, node)
	}

	node.Name = newName
	ft.nodes[uuid] = node
}

func (ft *FileTree) SetModtime(uuid Uuid, modtime time.Time) {
	node, exists := ft.nodes[uuid]
	if !exists {
		return
	}
	node.Modtime = modtime
	ft.nodes[uuid] = node
}

func (ft FileTree) GetNode(uuid Uuid) (FileTreeNode, bool) {
	node, exists := ft.nodes[uuid]
	if !exists {
		return FileTreeNode{}, false
	}

	return FileTreeNode{
		Uuid:    uuid,
		Name:    node.Name,
		Hash:    node.Hash,
		Modtime: node.Modtime,
		IsDir:   node.IsDir,
		Parent:  node.getParentUuid(),
	}, true
}

func (ft *FileTree) GetPath(uuid Uuid) (string, bool) {
	node, exists := ft.nodes[uuid]
	if !exists {
		return "", false
	}

	return node.GetPath(), true
}

func (ft *FileTree) GetParents(uuid Uuid) ([]Uuid, bool) {
	var parents []Uuid
	currentParent := ft.nodes[uuid].Parent
	for currentParent != nil {
		parents = append([]Uuid{currentParent.Uuid}, parents...)
		currentParent = currentParent.Parent
	}
	return parents, true
}

func (ft FileTree) GetPathToUuidMap() map[string]Uuid {
	var paths = make(map[string]Uuid)
	for uuid := range ft.nodes {
		paths[ft.nodes[uuid].GetPath()] = uuid
	}
	return paths
}
