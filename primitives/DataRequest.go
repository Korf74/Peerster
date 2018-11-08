package primitives

type DataRequest struct {
	Origin string
	Destination string
	HopLimit uint32
	HashValue []byte
}
