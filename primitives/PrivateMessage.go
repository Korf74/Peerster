package primitives

type PrivateMessage struct {
	Origin string
	ID uint32
	Text string
	Destination string
	HopLimit uint32
}

