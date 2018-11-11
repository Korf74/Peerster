package primitives

type File struct {
	Name string
	Size int64
	First *FileElement
	Complete bool
}

type FileElement struct {
	Next *FileElement
	Hash string
	Name string
	File *File
}


