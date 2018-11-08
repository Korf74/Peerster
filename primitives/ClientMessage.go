package primitives

type ClientMessage struct {
	Text string
	Rumor bool
	Private bool
	DataRequest *DataRequest
	NewFile string
	To string
}
