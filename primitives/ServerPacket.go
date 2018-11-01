package primitives

type ServerPacket struct {
	Origin string
	Content string
	Peers *[]string
	Contacts *[]string
	Private bool
	Flush bool
}
