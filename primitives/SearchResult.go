package primitives

type SearchResult struct {
	FileName string
	MetafileHash []byte
	ChunkMap []uint64
}
