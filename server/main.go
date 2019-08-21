package main

import (
	"bytes"
	"encoding/json"
	"github.com/Korf74/Peerster/utils"
	"github.com/dedis/protobuf"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)
var server *ServerInfo

type Peer string

type ServerInfo struct {
	Addr *net.UDPAddr
	MsgBuffer []peerMessage
	Peers []string
}

type updateMessage struct {
	Messages []peerMessage
}

type setupMessage struct {
	Addr string
	Peers []string
}

type downloadMessage struct {
	MetaHash string `json:"metaHash"`
	Contact string `json:"contact"`
	FileName string `json:"fileName"`
	GossipID int `json:"id"`
}

type peerMessage struct {
	Origin string
	Content string
}

type clientMessage struct {
	Text string `json:"message"`
	To string `json:"to"`
}

type ChatMessage struct {
	Text string
}

type addPeerMessage struct {
	Peer string `json:"peer"`
	GossipID int `json:"id"`
}

type createGossiperMessage struct {
	ID int
	Addr string
}

type idMessage struct {
	GossipID int `json:"id"`
}

type gossiperAddrMessage struct {
	GossipAddr string `json:"gossiperAddress"`
}

func getMessages(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	data, err := json.Marshal(updateMessage{server.MsgBuffer})

	utils.CheckError(err)

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(data)
	utils.CheckError(err)

	server.MsgBuffer = make([]peerMessage, 0, 100)

}

func getPeers(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	data, err := json.Marshal(setupMessage{server.Addr.IP.String(), server.Peers})

	utils.CheckError(err)

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(data)
	utils.CheckError(err)

}

func newMsg(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = clientMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	var pckt = ChatMessage{}

	pckt.Text = msg.Text

	var packetBytes, err4 = protobuf.Encode(&pckt)
	utils.CheckError(err4)

	peerAddr, err := net.ResolveUDPAddr("udp4", msg.To+":5001")
	utils.CheckError(err)

	var udpConn, err5 = net.DialUDP("udp4", nil, peerAddr)
	utils.CheckError(err5)

	var _, err6 = udpConn.Write(packetBytes)
	utils.CheckError(err6)

	udpConn.Close()

}

func waitForMessages() {

	var connection, err = net.ListenUDP("udp4", &net.UDPAddr{server.Addr.IP, 5001, server.Addr.Zone})
	utils.CheckError(err)

	defer connection.Close()

	var buffer = make([]byte, 2048)

	for {

		var sz, from, errRcv = connection.ReadFromUDP(buffer)
		utils.CheckError(errRcv)

		if errRcv == nil {

			var pckt = &ChatMessage{}

			var errDecode = protobuf.Decode(buffer[:sz], pckt)
			utils.CheckError(errDecode)

			if pckt.Text != "" {
				server.MsgBuffer = append(server.MsgBuffer,
					peerMessage{from.IP.String(), pckt.Text})
			}


		}

	}

}
/*
func createGossiper(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = gossiperAddrMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	var id = len(gossipers)

	var gossiper = gossipInfo{}

	uiPort := strconv.Itoa(8080 + id + 1)
	gossipAddr := msg.GossipAddr
	gossipPort := strconv.Itoa(5000 + id)
	udpAddrGossiper := gossipAddr+":"+gossipPort

	_, err = net.ResolveUDPAddr("udp4", udpAddrGossiper)

	var data []byte

	if err != nil {

		data, err = json.Marshal(createGossiperMessage{-1, ""}) // TODO concurrency
		utils.CheckError(err)

	} else {

		name := "GossiperGUI"+strconv.Itoa(id)

		g, channel := gossip.NewGossiper(uiPort, udpAddrGossiper, name,
			"", 10, false) // TODO rtimer

		gossiper.Contacts = append(gossiper.Contacts, name)

		gossiper.Channel = channel
		gossiper.G = g

		go waitForMessages(&gossiper, channel)

		udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+uiPort)
		utils.CheckError(err)

		gossiper.Addr = udpAddr

		gossipers = append(gossipers, &gossiper)

		data, err = json.Marshal(createGossiperMessage{id, udpAddrGossiper}) // TODO concurrency
		utils.CheckError(err)
	}

	w.Header().Set("Content-Type", "application/json")

	w.Write(data)

}*/

func main() {

	//gossipers = make([]*gossipInfo, 0, 100)

	server = &ServerInfo{}

	server.Peers = os.Args[1:]
	udpAddr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:5000")
	utils.CheckError(err)

	go waitForMessages()

	server.Addr = udpAddr

	r := mux.NewRouter()

	r.Methods("POST").Subrouter().HandleFunc("/newMessage", newMsg)//HandleFunc("/", newMsg)
	r.Methods("GET").Subrouter().HandleFunc("/getMessages", getMessages)//HandleFunc("/", newMsg)
	r.Methods("GET").Subrouter().HandleFunc("/getPeers", getPeers)//HandleFunc("/", newMsg)
	r.Handle("/", http.FileServer(http.Dir(".")))

	log.Println(http.ListenAndServe(":8080", r))

}
