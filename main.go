package main

import (
	"flag"
	"github.com/Korf74/Peerster/gossip"
	"github.com/Korf74/Peerster/utils"
	"strconv"
)

/**
Your ​ gossiper program takes as arguments the two ports ​ UIPort
and ​ gossipAddr​ , as well as a list of one or more addresses of other gossipers it knows of
at the beginning, in the form ​ ip1:port1,ip2:port2,etc (note that list elements are
separated by a comma). Pay attention: using ports below 1024 requires root privileges and
quite a few of them are reserved​ , thus please use higher ports. It also needs a flag “simple”
to enforce simple broadcast mode for compatibility. ​ You need to follow this input format in
order to comply with automatic testing, otherwise tests will fail. You may use ​ the flag
package​ from go's standard library to implement your CLIs.


./gossiper -UIPort=10000 -gossipAddr=127.0.0.1:5000 -name=nodeA
-peers=127.0.0.1:5001,10.1.1.7:5002 -simple
 */


 func main() {

 	var UIPort = flag.String("UIPort", "8080",
 		"port for the UI client")

 	var gossipAddr = flag.String("gossipAddr", "127.0.0.1:5000",
 		"ip:port for the gossiper")

 	var name = flag.String("name", "Lambda",
 		"name of the gossiper")

 	var peers = flag.String("peers", "",
 		"comma separated list of peers of the form ip:port")

 	var rtimer = flag.String("rtimer", "0",
 		"route rumors sending period in seconds, 0 to disable sending of " +
 		"route rumors")

 	var simple = flag.Bool("simple", false,
 		"run gossiper in simple broadcast mode")

 	flag.Parse()

 	/*fmt.Println("uiport : ", *UIPort)
 	fmt.Println("gossipaddr : ", *gossipAddr)
 	fmt.Println("name : ", *name)
 	fmt.Println("peers : ", *peers)
 	fmt.Println("simple : ", *simple)*/

	 var timer, err = strconv.Atoi(*rtimer)
	 utils.CheckError(err)

 	gossip.NewGossiper(*UIPort, *gossipAddr, *name, *peers,  timer, *simple)

 	for {

	}


 }