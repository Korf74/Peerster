package primitives

type ServerPacket struct {
	Origin string
	Content string
	Peers *[]string
	Flush bool
}
