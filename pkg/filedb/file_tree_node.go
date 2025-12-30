package filedb

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type Uuid [8]uint64

var NilUuid Uuid

func UuidFromString(s string) Uuid {
	var u Uuid
	bytes := make([]byte, 64)
	copy(bytes, s)
	for i := 0; i < 8; i++ {
		u[i] = binary.LittleEndian.Uint64(bytes[i*8 : (i+1)*8])
	}
	return u
}

func (u Uuid) String() string {
	var bytes [64]byte
	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint64(bytes[i*8:(i+1)*8], u[i])
	}

	var end int
	for i, b := range bytes {
		if b == 0 {
			end = i
			break
		}
	}

	return string(bytes[:end])
}

func (u Uuid) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *Uuid) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = UuidFromString(s)
	return nil
}

func CompareUuids(a, b *Uuid) int {
	for i := range 8 {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

type FileName string

func FileNameFromString(s string) FileName {
	return FileName(s)
}

func (fn FileName) String() string {
	return string(fn)
}

func (u FileName) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *FileName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = FileNameFromString(s)
	return nil
}

type Hash [32]byte

func (u Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = HashFromString(s)
	return nil
}

func HashFromString(s string) Hash {
	hexBytes, _ := hex.DecodeString(s)
	var h Hash
	copy(h[:], hexBytes)
	return h
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

type FileTreeNode struct {
	Uuid    Uuid
	Name    FileName
	Hash    Hash
	Modtime time.Time
	IsDir   bool
	Parent  Uuid
}

func (i FileTreeNode) SortString() string {
	return i.Uuid.String()
}

type fileTreeNodeInternal struct {
	Uuid     Uuid
	Name     FileName
	Hash     Hash
	Modtime  time.Time
	IsDir    bool
	Parent   *fileTreeNodeInternal
	Children []*fileTreeNodeInternal
	path     string
}

func (n *fileTreeNodeInternal) getParentUuid() Uuid {
	if n.Parent == nil {
		return NilUuid
	}
	return n.Parent.Uuid
}

func (n *fileTreeNodeInternal) GetPath() string {
	if n.path == "" {
		parts := n.getPathParts()
		n.path = strings.Join(parts, "/")
	}
	return n.path
}

func (n *fileTreeNodeInternal) invalidatePathCache() {
	n.path = ""
	for _, child := range n.Children {
		child.invalidatePathCache()
	}
}

func (n *fileTreeNodeInternal) getPathParts() []string {
	if n.Parent != nil {
		return append(n.Parent.getPathParts(), n.Name.String())
	} else {
		return []string{n.Name.String()}
	}
}
