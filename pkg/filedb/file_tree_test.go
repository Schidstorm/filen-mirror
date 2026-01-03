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
