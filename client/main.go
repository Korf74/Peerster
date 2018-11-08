package main

import (
	"encoding/hex"
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

	var request = flag.String("request", "",
		"request a chunk or metafile of this hash")

	flag.Parse()

	var udpAddrRemote, err1 = net.ResolveUDPAddr("udp4", "127.0.0.1:"+*UIPort)
	utils.CheckError(err1)

	var udpConn, err3 = net.DialUDP("udp4", nil, udpAddrRemote)
	utils.CheckError(err3)

	var pckt = primitives.ClientMessage{}

	pckt.Text = *msg

	if *file != "" {
		pckt.NewFile = *file
	} else if *dest != "" {

		if *request != "" {
			if len(*request) != 64 {
				fmt.Println("Incorrect hash")
				return
			}

			hash, err := hex.DecodeString(*request)
			utils.CheckError(err)

			pckt.DataRequest = &primitives.DataRequest{
				Destination: *dest,
				HashValue: hash,
			}

			fmt.Println("sending data req")

		} else {
			pckt.Private = true
		}
		pckt.To = *dest
	} else {
		pckt.Rumor = true
	}

	var packetBytes, err4 = protobuf.Encode(&pckt)
	utils.CheckError(err4)

	var _, err5 = udpConn.Write(packetBytes)
	utils.CheckError(err5)

	udpConn.Close()
}
