package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/Korf74/Peerster/primitives"
	"github.com/Korf74/Peerster/utils"
	"github.com/dedis/protobuf"
	"net"
	"strconv"
	"strings"
)

func main() {

	var UIPort = flag.String("UIPort", "8080",
		"port for the UI client")

	var dest = flag.String("dest", "",
		"destination for the private message")

	var file = flag.String("file", "",
		"file to be indexed by the gossiper, or filename of the requested file")

	var msg = flag.String("msg", "",
		"message to be sent")

	var request = flag.String("request", "",
		"request a chunk or metafile of this hash")

	var keywords = flag.String("keywords", "",
		"keywords for file search")

	var budget = flag.String("budget", "",
		"budget of file search")

	flag.Parse()

	var udpAddrRemote, err1 = net.ResolveUDPAddr("udp4", "127.0.0.1:"+*UIPort)
	utils.CheckError(err1)

	var udpConn, err3 = net.DialUDP("udp4", nil, udpAddrRemote)
	utils.CheckError(err3)

	var pckt = primitives.ClientMessage{}

	pckt.Text = *msg
	pckt.FileName = *file

	if *file != "" && *request == "" {
		pckt.NewFile = *file
	} else if *dest == "" && *file != "" && *request != "" {

		if *request != "" && *file != "" {
			if len(*request) != 64 {
				fmt.Println("Incorrect hash")
				return
			}

			hash, err := hex.DecodeString(*request)
			utils.CheckError(err)

			pckt.DataRequest = &primitives.DataRequest{
				HashValue: hash,
			}

			fmt.Println("sending data req")

		}

	} else if *dest != "" {

		if *request != "" && *file != "" {
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
	} else if *keywords != "" {

		var budgetInt int

		if *budget == "" {
			budgetInt = 2
		} else {
			var err error
			budgetInt, err = strconv.Atoi(*budget)
			utils.CheckError(err)
		}

		keywordArray := strings.Split(*keywords, ",")

		pckt.Keywords = keywordArray
		pckt.Budget = uint64(budgetInt)


	} else {
		pckt.Rumor = true
	}

	var packetBytes, err4 = protobuf.Encode(&pckt)
	utils.CheckError(err4)

	var _, err5 = udpConn.Write(packetBytes)
	utils.CheckError(err5)

	udpConn.Close()
}
