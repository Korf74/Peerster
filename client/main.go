package main

import (
	"flag"
	"fmt"
	"github.com/Korf74/Peerster/primitives"
	"github.com/Korf74/Peerster/utils"
	"github.com/dedis/protobuf"
	"net"
)

func main() {

	var UIPort = flag.String("UIPort", "8080",
		"port for the UI client")

	var dest = flag.String("dest", "",
		"destination for the private message")

	var file = flag.String("file", "",
		"file to be indexed by the gossiper")

	var msg = flag.String("msg", "",
		"message to be sent")

	flag.Parse()

	var udpAddrRemote, err1 = net.ResolveUDPAddr("udp4", "127.0.0.1:"+*UIPort)
	utils.CheckError(err1)

	var udpConn, err3 = net.DialUDP("udp4", nil, udpAddrRemote)
	utils.CheckError(err3)

	var pckt = primitives.ClientMessage{}

	var filePath = "./_SharedFiles/"+*file;
	fmt.Println(filePath)

	pckt.Text = *msg

	if *dest != "" {
		pckt.Private = true
		pckt.To = *dest
	} else {
		pckt.Private = false
	}

	var packetBytes, err4 = protobuf.Encode(&pckt)
	utils.CheckError(err4)

	var _, err5 = udpConn.Write(packetBytes)
	utils.CheckError(err5)

	udpConn.Close()
}
