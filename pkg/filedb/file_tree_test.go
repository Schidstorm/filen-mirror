package filedb_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Schidstorm/edge_config/apps/filen-mirror/pkg/filedb"
	"github.com/stretchr/testify/assert"
)

var benchmarkTree1 *filedb.FileTree
var benchmarkTree2 *filedb.FileTree

func init() {
	benchmarkTree1 = filedb.NewFileTree()
	loadTestTree(nil, benchmarkTree1)

	benchmarkTree2 = filedb.NewFileTree()
	loadTestTree(nil, benchmarkTree2)
}

func TestDiffRemovedFile(t *testing.T) {
	tree1 := generateTestTree()
	tree2 := generateTestTree()

	tree2.Remove(filedb.UuidFromString("file1"))
	var numDiffs int
	for diffItem := range filedb.StartDiff(tree1, tree2) {
		numDiffs++
		d, ok := diffItem.(filedb.DiffRemoved)
		assert.True(t, ok)
		assert.Equal(t, filedb.UuidFromString("file1"), d.Uuid)
	}
	assert.Equal(t, 1, numDiffs)
}

func TestDiffRemovedDir(t *testing.T) {
	tree1 := generateTestTree()
	tree2 := generateTestTree()

	tree2.Remove(filedb.UuidFromString("dir2"))
	var numDiffs int
	for diffItem := range filedb.StartDiff(tree1, tree2) {
		numDiffs++
		d, ok := diffItem.(filedb.DiffRemoved)
		assert.True(t, ok)
		assert.True(t, d.Uuid == filedb.UuidFromString("dir2") || d.Uuid == filedb.UuidFromString("file1"))
	}
	assert.Equal(t, 2, numDiffs)
}

func TestDiffMoveDir(t *testing.T) {
	tree1 := generateTestTree()
	tree2 := generateTestTree()

	tree2.Move(filedb.UuidFromString("dir2"), filedb.NilUuid, "moved-dir")
	var numDiffs int
	for diffItem := range filedb.StartDiff(tree1, tree2) {
		numDiffs++
		d, ok := diffItem.(filedb.DiffModified)
		assert.True(t, ok)
		assert.Equal(t, filedb.UuidFromString("dir2"), d.Uuid)
		assert.Equal(t, "dir1/dir2", d.OldPath)
		assert.Equal(t, "moved-dir", d.NewPath)
	}
	assert.Equal(t, 1, numDiffs)
}

func TestGetPathRebuildsCacheAfterAncestorMove(t *testing.T) {
	tree := generateTestTree()
	fileUuid := filedb.UuidFromString("file1")

	path, ok := tree.GetPath(fileUuid)
	assert.True(t, ok)
	assert.Equal(t, "dir1/dir2/file1.txt", path)

	tree.Move(filedb.UuidFromString("dir1"), filedb.NilUuid, "renamed-dir")

	path, ok = tree.GetPath(fileUuid)
	assert.True(t, ok)
	assert.Equal(t, "renamed-dir/dir2/file1.txt", path)
}

func TestDiffResultsAreSortedByUuid(t *testing.T) {
	tree1 := filedb.NewFileTree()
	tree1.CreateFile(filedb.UuidFromString("a"), filedb.NilUuid, "a", time.Unix(0, 0), "")
	tree1.CreateFile(filedb.UuidFromString("c"), filedb.NilUuid, "c", time.Unix(0, 0), "")

	tree2 := filedb.NewFileTree()
	tree2.CreateFile(filedb.UuidFromString("b"), filedb.NilUuid, "b", time.Unix(0, 0), "")
	tree2.CreateFile(filedb.UuidFromString("c"), filedb.NilUuid, "c", time.Unix(1, 0), "")

	var uuids []filedb.Uuid
	for diffItem := range filedb.StartDiff(tree1, tree2) {
		switch item := diffItem.(type) {
		case filedb.DiffAdded:
			uuids = append(uuids, item.Uuid)
		case filedb.DiffRemoved:
			uuids = append(uuids, item.Uuid)
		case filedb.DiffModified:
			uuids = append(uuids, item.Uuid)
		default:
			t.Fatalf("unexpected diff item type %T", diffItem)
		}
	}

	assert.Equal(t, []filedb.Uuid{
		filedb.UuidFromString("a"),
		filedb.UuidFromString("b"),
		filedb.UuidFromString("c"),
	}, uuids)
}

func TestCopyFromPreservesSelfAndIndependentMaps(t *testing.T) {
	source := generateTestTree()
	source.CopyFrom(source)
	assert.Contains(t, source.GetPathToUuidMap(), "dir1/dir2/file1.txt")

	destination := filedb.NewFileTree()
	destination.CopyFrom(source)
	destination.Remove(filedb.UuidFromString("file1"))
	_, sourceStillHasFile := source.GetNode(filedb.UuidFromString("file1"))
	assert.True(t, sourceStillHasFile)
}

func BenchmarkDiff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for range filedb.StartDiff(benchmarkTree1, benchmarkTree2) {

		}
	}
}

func BenchmarkDiffWithNil(b *testing.B) {
	nilTree := filedb.NewFileTree()
	for i := 0; i < b.N; i++ {
		for range filedb.StartDiff(benchmarkTree1, nilTree) {
		}
	}
}

func BenchmarkCopyFrom(b *testing.B) {
	nilTree := filedb.NewFileTree()
	for i := 0; i < b.N; i++ {
		nilTree.CopyFrom(benchmarkTree1)
	}
}

type FileTreeNode struct {
	Uuid    filedb.Uuid
	Name    filedb.FileName
	Hash    filedb.Hash
	Modtime time.Time
	IsDir   bool
	Parent  filedb.Uuid
}

func loadTestTree(t *testing.T, tree *filedb.FileTree) {
	b, err := os.ReadFile("test-nodes.json")
	assert.NoError(t, err)
	var nodes []filedb.FileTreeNode
	err = json.Unmarshal(b, &nodes)
	assert.NoError(t, err)

	tree.EnsureItems(nodes)
}

func generateTestTree() *filedb.FileTree {
	tree := filedb.NewFileTree()

	tree.CreateDir(filedb.UuidFromString("dir1"), filedb.NilUuid, "dir1")
	tree.CreateDir(filedb.UuidFromString("dir2"), filedb.UuidFromString("dir1"), "dir2")
	tree.CreateFile(filedb.UuidFromString("file1"), filedb.UuidFromString("dir2"), "file1.txt", time.Unix(0, 0), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	return tree
}
