package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Korf74/Peerster/gossip"
	"github.com/Korf74/Peerster/primitives"
	"github.com/Korf74/Peerster/utils"
	"github.com/dedis/protobuf"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)
var gossipers []*gossipInfo

type gossipInfo struct {
	G *gossip.Gossiper
	Channel chan *primitives.ServerPacket
	Addr *net.UDPAddr
	MsgBuffer []peerMessage
	PrivateMsgBuffer []peerMessage
	Peers []string
	NewPeers []string
	Contacts []string
	NewContacts []string
}

type updateMessage struct {
	Messages []peerMessage
	PrivateMessages []peerMessage
	Peers []string
	Contacts []string
}

type peerMessage struct {
	Origin string
	Content string
}

type clientMessage struct {
	Text string `json:"message"`
	To string `json:"to"`
	GossipID int `json:"id"`
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

func update(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = idMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	gossiper := gossipers[msg.GossipID]

	data, err := json.Marshal(updateMessage{
		gossiper.MsgBuffer,
	gossiper.PrivateMsgBuffer, gossiper.NewPeers,
	gossiper.NewContacts})

	utils.CheckError(err)

	if data == nil {
		fmt.Println("ASDFADFASDFSAFDSADFASFSDFSADFASDFASDFSADFSDF")
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(data)
	utils.CheckError(err)

	gossiper.Channel <- &primitives.ServerPacket{Flush:true}

}

func newMsg(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = clientMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	var pckt = primitives.ClientMessage{}

	pckt.Text = msg.Text

	if msg.To != "rumor" {
		pckt.To = msg.To
		pckt.Private = true
	}


	var packetBytes, err4 = protobuf.Encode(&pckt)
	utils.CheckError(err4)

	gossiper := gossipers[msg.GossipID]

	var udpConn, err5 = net.DialUDP("udp4", nil, gossiper.Addr)
	utils.CheckError(err5)

	var _, err6 = udpConn.Write(packetBytes)
	utils.CheckError(err6)

	udpConn.Close()

}

func newPeer(w http.ResponseWriter, r *http.Request) {

	var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = addPeerMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	peerAddr, err := net.ResolveUDPAddr("udp4", msg.Peer)
	utils.CheckError(err)

	gossipers[msg.GossipID].G.NotifyNewPeer(peerAddr)

}

func newFile(w http.ResponseWriter, r *http.Request) {

	/*var body, err = ioutil.ReadAll(r.Body)
	utils.CheckError(err)

	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))

	var msg = idMessage{}
	err = json.Unmarshal(body, &msg)
	utils.CheckError(err)

	fmt.Println(msg.GossipID)*/

	fmt.Println(r.FormValue("id"))

	file, handler, err := r.FormFile("file")
	utils.CheckError(err)
	defer file.Close()

	// copy example
	f, err := os.OpenFile("../_SharedFiles/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	utils.CheckError(err)
	defer f.Close()
	io.Copy(f, file)

	id, err := strconv.Atoi(r.FormValue("id"))
	utils.CheckError(err)

	gossipers[id].G.NotifyFile(f.Name())

}

func hasPeer(peer string, gossiper *gossipInfo) bool {

	for _, p := range gossiper.Peers {
		if peer == p {
			return true
		}
	}

	return false

}

func hasContact(contact string, gossiper *gossipInfo) bool {

	for _, c := range gossiper.Contacts {
		if contact == c {
			return true
		}
	}

	return false

}

func waitForMessages(gossiper *gossipInfo, channel chan *primitives.ServerPacket) {

	gossiper.MsgBuffer = make([]peerMessage, 0, 100)
	gossiper.PrivateMsgBuffer = make([]peerMessage, 0, 100)
	gossiper.Peers = make([]string, 0, 100)
	gossiper.NewPeers = make([]string, 0, 100)
	gossiper.Contacts = make([]string, 0, 100)
	gossiper.NewContacts = make([]string, 0, 100)

	for {
		pckt := <- channel

		if pckt.Flush {
			gossiper.MsgBuffer = make([]peerMessage, 0, 100)
			gossiper.PrivateMsgBuffer = make([]peerMessage, 0, 100)
			gossiper.NewPeers = make([]string, 0, 100)
			gossiper.NewContacts = make([]string, 0, 100)
		} else {

			for _, peer := range *pckt.Peers {
				if !hasPeer(peer, gossiper) {
					gossiper.Peers = append(gossiper.Peers, peer)
					gossiper.NewPeers = append(gossiper.NewPeers, peer)
				}
			}

			for _, contact := range *pckt.Contacts {
				if !hasContact(contact, gossiper) {
					gossiper.Contacts = append(gossiper.Contacts, contact)
					gossiper.NewContacts = append(gossiper.NewContacts, contact)
				}
			}

			if pckt.Origin != "" && pckt.Content != "" {
				if pckt.Private {
					gossiper.PrivateMsgBuffer = append(gossiper.MsgBuffer,
						peerMessage{pckt.Origin, pckt.Content})
				} else {
					gossiper.MsgBuffer = append(gossiper.MsgBuffer,
						peerMessage{pckt.Origin, pckt.Content})
				}
			}
		}

	}

}

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

		g, channel := gossip.NewGossiper(uiPort, udpAddrGossiper, "GossiperGUI"+strconv.Itoa(id),
			"", 10, false) // TODO rtimer

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

}

func main() {

	gossipers = make([]*gossipInfo, 0, 100)

	r := mux.NewRouter()

	r.Methods("POST").Subrouter().HandleFunc("/", newMsg)//HandleFunc("/", newMsg)
	r.Methods("POST").Subrouter().HandleFunc("/newPeer", newPeer)//HandleFunc("/", newMsg)
	r.Methods("POST").Subrouter().HandleFunc("/getMessages", update)//HandleFunc("/", newMsg)
	r.Methods("POST").Subrouter().HandleFunc("/createGossiper", createGossiper)//HandleFunc("/", newMsg)
	r.Methods("POST").Subrouter().HandleFunc("/newFile", newFile)//HandleFunc("/", newMsg)
	r.Handle("/", http.FileServer(http.Dir(".")))

	log.Println(http.ListenAndServe(":8080", r))

}
