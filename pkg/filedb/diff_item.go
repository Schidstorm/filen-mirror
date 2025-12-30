package filedb

type DiffItemType int

const (
	DiffItemTypeAdded DiffItemType = iota
	DiffItemTypeRemoved
	DiffItemTypeModified
)

type DiffItem interface {
	Type() DiffItemType
}

type DiffAdded struct {
	Uuid Uuid
	Path string
}

func (n DiffAdded) Type() DiffItemType {
	return DiffItemTypeAdded
}

type DiffRemoved struct {
	Uuid Uuid
	Path string
}

func (n DiffRemoved) Type() DiffItemType {
	return DiffItemTypeRemoved
}

type DiffModified struct {
	Uuid    Uuid
	OldPath string
	NewPath string
}

func (n DiffModified) Type() DiffItemType {
	return DiffItemTypeModified
}
