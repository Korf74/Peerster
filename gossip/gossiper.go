package gossip

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Korf74/Peerster/primitives"
	"github.com/Korf74/Peerster/utils"
	"github.com/dedis/protobuf"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SCIPER = "249996"
const SHARED_FILES_PREFIX = "_SharedFiles/"
const DOWNLOADED_FILES_PREFIX = "_Downloads/"
const MAX_PEERS = 2048
const MAX_PACKET_SIZE = CHUNK_SIZE + 1024
const MAX_MSG = 2048
const MAX_CHANNEL_BUFFER = 2048
const HOP_LIMIT = 10
const MAX_FILES = 100
const MAX_WAITING_DATA_ELENENTS = 300
const CHUNK_SIZE = 8000

/* TODO IDEA : a function that receives channels and statuses and pair them, if new status
** TODO wihout chennel -> create
 */

type WaitingDataElement struct {
	Channel chan *primitives.DataReply
	elem *primitives.FileElement
}

type Gossiper struct {
	addr *net.UDPAddr
	conn *net.UDPConn
	clientAddr *net.UDPAddr
	clientConn *net.UDPConn
	name string
	simple bool
	peers []net.UDPAddr
	status map[string]*primitives.PeerStatus
	waitingChannels map[string]chan *primitives.StatusPacket
	waitingThreads map[string]bool
	receivedMsgs map[string][]primitives.RumorMessage
	receivedPrivateMsgs map[string][]primitives.PrivateMessage
	routingTable map[string]*net.UDPAddr
	fileElements map[string]*primitives.FileElement
	nextID uint32
	dataRepliesChannel chan *primitives.DataReply
	waitingDataReplies []WaitingDataElement
	newMsgChannel chan *primitives.ServerPacket
	muxPeers sync.Mutex // TODO REALLY NEED IT ? THREAD BY PEER
	muxStatus sync.Mutex
	muxChannels sync.Mutex // TODO SAME
	muxThreads sync.Mutex
	muxMsgs sync.Mutex
	muxDataReplies sync.Mutex

}

func NewGossiper(clientPort, address, gossiperName, peers string, rtimer int, simple bool) (g *Gossiper, channel chan *primitives.ServerPacket) {

	var udpAddrGossip, err1 = net.ResolveUDPAddr("udp4", address)
	utils.CheckError(err1)

	var udpConnGossip, err2 = net.ListenUDP("udp4", udpAddrGossip)
	utils.CheckError(err2)

	var udpAddrClient, err3 = net.ResolveUDPAddr("udp4", "127.0.0.1:"+clientPort)
	utils.CheckError(err3)

	var udpConnClient, err4 = net.ListenUDP("udp4", udpAddrClient)
	utils.CheckError(err4)

	var peersList []net.UDPAddr

	if len(peers) != 0 {
		peersList = make_peers(strings.Split(peers, ","))
	}

	channel = make(chan *primitives.ServerPacket, MAX_MSG)

	g =  &Gossiper {
		addr: udpAddrGossip,
		conn: udpConnGossip,
		clientAddr: udpAddrClient,
		clientConn: udpConnClient,
		name: gossiperName,// TODO+"_"+SCIPER,
		simple: simple,
		peers: peersList,
		status: make(map[string]*primitives.PeerStatus),
		waitingChannels: make(map[string]chan *primitives.StatusPacket),
		waitingThreads: make(map[string]bool),
		receivedMsgs: make(map[string][]primitives.RumorMessage),
		receivedPrivateMsgs: make(map[string][]primitives.PrivateMessage),
		routingTable: make(map[string]*net.UDPAddr),
		fileElements: make(map[string]*primitives.FileElement),
		nextID: 1,
		dataRepliesChannel: make(chan *primitives.DataReply),
		waitingDataReplies: make([]WaitingDataElement, 0, MAX_WAITING_DATA_ELENENTS),
		newMsgChannel: channel,
		muxPeers: sync.Mutex{},
		muxStatus: sync.Mutex{},
		muxThreads: sync.Mutex{},
		muxChannels: sync.Mutex{},
		muxMsgs: sync.Mutex{},
	}

	g.routingTable[g.name] = g.addr

	channel <- g.makeServerPacket(nil)

	go g.listenPeers(simple)
	//go g.antiEntropy()
	go g.listenClient(simple)
	if rtimer > 0 { go g.sendRouteRumorMessages(rtimer) }
	go g.listenDataReplies()

	return
}

func (g *Gossiper) lockPeers() {
	g.muxPeers.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockPeers() {
	g.muxPeers.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g *Gossiper) lockStatus() {
	g.muxStatus.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockStatus() {
	g.muxStatus.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g *Gossiper) lockThreads() {
	g.muxThreads.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockThreads() {
	g.muxThreads.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g *Gossiper) lockChannels() {
	g.muxChannels.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockChannels() {
	g.muxChannels.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g *Gossiper) lockMsgs() {
	g.muxMsgs.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockMsgs() {
	g.muxMsgs.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g *Gossiper) lockDataReplies() {
	g.muxDataReplies.Lock()
	//fmt.Println(g.addr.String()+" LOCKED")
}

func (g *Gossiper) unlockDataReplies() {
	g.muxDataReplies.Unlock()
	//fmt.Println(g.addr.String()+" UNLOCKED")
}

func (g * Gossiper) removeDataReply(element *WaitingDataElement) {

	g.lockDataReplies()

	sz := len(g.waitingDataReplies)

	for i, e := range g.waitingDataReplies {

		if e == *element {

			if i + 1 < sz {
				g.waitingDataReplies = append(g.waitingDataReplies[:i], g.waitingDataReplies[i+1:]...)
			} else {
				g.waitingDataReplies = g.waitingDataReplies[:i]
			}

			break

		}

	}

	g.unlockDataReplies()

}

func (g *Gossiper) getElem(hash string) (ret *primitives.FileElement) {

	g.lockDataReplies()

	for _, e := range g.waitingDataReplies {

		if e.elem.Hash == hash {

			ret = e.elem

			break

		}

	}

	g.unlockDataReplies()

	return

}

func (g *Gossiper) updateDataReply(hash string) (ret string) {

	g.lockDataReplies()

	sz := len(g.waitingDataReplies)

	for i, e := range g.waitingDataReplies {

		if e.elem.Hash == hash {

			elem := e.elem

			if elem.Next != nil {

				fmt.Println("Waiting updated : "+hash+" -> "+e.elem.Next.Hash)

				g.waitingDataReplies[i].elem = e.elem.Next

				ret = e.elem.Next.Hash

			} else { // remove if last

				if i + 1 < sz {
					g.waitingDataReplies = append(g.waitingDataReplies[:i], g.waitingDataReplies[i+1:]...)
				} else {
					g.waitingDataReplies = g.waitingDataReplies[:i]
				}
			}

			break

		}

	}

	g.unlockDataReplies()

	return

}

func (g * Gossiper) appendDataReply(element *WaitingDataElement) {

	g.lockDataReplies()

	g.waitingDataReplies = append(g.waitingDataReplies, *element)

	g.unlockDataReplies()

}


/*func (g *Gossiper) getStatus(peer string) {

	g.lock()
	var _, exists = g.status[peer]

	if !exists {
		g.status[peer] = &primitives.PeerStatus{
			Identifier: peer,
			NextID: 1,
		}
	}

	g.unlock()

}*/

func (g *Gossiper) checkMsgStatus(peer string) {

	g.checkMsgs(peer)

	g.lockStatus()

	var _, exists = g.status[peer]

	if !exists {
		g.status[peer] = &primitives.PeerStatus{
			Identifier: peer,
			NextID: 1,
		}
	}

	g.unlockStatus()

}

/*func (g *Gossiper) checkStatus(peer string) {

	g.checkMsgStatus(peer)

	g.lock()

	var _, exists = g.status[peer]

	if !exists {
		g.status[peer] = &primitives.PeerStatus{
			Identifier: peer,
			NextID: 1,
		}
	}

	g.unlock()

}*/

func (g *Gossiper) checkChannel(peer string) {

	g.lockChannels()

	var _, exists = g.waitingChannels[peer]

	if !exists {
		g.waitingChannels[peer] = make(chan *primitives.StatusPacket, MAX_CHANNEL_BUFFER)
	}

	g.unlockChannels()

}

func (g *Gossiper) getChannel(peer string) (channel chan *primitives.StatusPacket) {

	g.checkChannel(peer)

	g.lockChannels()

	channel = g.waitingChannels[peer]

	g.unlockChannels()

	return

}

func (g *Gossiper) checkThread(peer string) {

	g.lockThreads()

	var _, exists = g.waitingThreads[peer]

	if !exists {
		g.waitingThreads[peer] = false
	}

	g.unlockThreads()

}

func (g *Gossiper) getWaitingThread(peer string) (waiting bool) {

	g.checkThread(peer)

	g.lockThreads()

	waiting = g.waitingThreads[peer]

	g.unlockThreads()

	return

}

func (g *Gossiper) compareAndSwapThread(peer string, expected bool, newVal bool) (ret bool) {

	g.lockThreads()

	var _, exists = g.waitingThreads[peer]

	if !exists {
		g.waitingThreads[peer] = false
	}

	ret = g.waitingThreads[peer] == expected

	if ret {
		g.waitingThreads[peer] = newVal
		//fmt.Println("ACCESS TO THREAD GRANTED")
	} else {
		//fmt.Println("ACCES DENIED")
	}

	g.unlockThreads()

	return

}

func (g *Gossiper) setWaitingThread(peer string, val bool) {

	g.checkThread(peer)

	g.lockThreads()

	if val == false {
		//fmt.Println("ACCESS TO THREAD UNLOCKED")
	}

	g.waitingThreads[peer] = val

	g.unlockThreads()
}

func (g *Gossiper) checkMsgs(peer string) {

	g.lockMsgs()

	var _, exists = g.receivedMsgs[peer]

	if !exists {
		g.receivedMsgs[peer] = make([]primitives.RumorMessage, 0, MAX_MSG)
	}

	g.unlockMsgs()

}

func (g *Gossiper) setStatusID(peer string, id uint32) {

	g.checkMsgStatus(peer)

	g.lockStatus()

	g.status[peer].NextID = id

	g.unlockStatus()

}

func (g *Gossiper) incStatusID(peer string) {

	g.checkMsgStatus(peer)

	g.lockStatus()

	g.status[peer].NextID += 1

	g.unlockStatus()

}

func (g *Gossiper) getStatusID(peer string) (id uint32) {

	g.checkMsgStatus(peer)

	g.lockStatus()

	id = g.status[peer].NextID

	g.unlockStatus()

	return

}

/*func (g *Gossiper) checkPeerExists(peer string) {

	g.lock()

	_, exists = g.status[peer]

	g.unlock()

	return

}*/

func (g *Gossiper) appendMsg(peer string, msg *primitives.RumorMessage) {

	g.checkMsgStatus(peer)

	g.lockMsgs()

	g.receivedMsgs[peer] = append(g.receivedMsgs[peer], *msg)

	g.unlockMsgs()

}

func (g *Gossiper) getMsg(peer string, id uint32) (msg *primitives.RumorMessage) {

	g.checkMsgStatus(peer)

	g.lockMsgs()

	msg = &g.receivedMsgs[peer][id - 1]

	g.unlockMsgs()

	return

}

func (g *Gossiper) NotifyNewPeer(peer *net.UDPAddr) {
	g.addNewPeer(peer)
	fmt.Println("NEW PEER : "+peer.String())
	g.newMsgChannel <- g.makeServerPacket(nil)
}

func (g *Gossiper) addNewPeer(peer *net.UDPAddr) {

	g.lockPeers()

	g.peers = append(g.peers, *peer)

	g.unlockPeers()

}

func (g *Gossiper) getPeer(i int64) (peer *net.UDPAddr) {

	g.lockPeers()

	peer = &g.peers[i]

	g.unlockPeers()

	return

}

func (g* Gossiper) getRandomPeer() (peer *net.UDPAddr) {

	if len(g.peers) == 0 {
		return nil
	}

	var rd = rand.Int63n(int64(len(g.peers))) // TODO

	peer = g.getPeer(rd)

	return


}

func (g* Gossiper) getRandomPeerExcept(except *net.UDPAddr) (peer *net.UDPAddr) {

	if len(g.peers) <= 1 {
		return nil
	}

	peer = except

	for peer == except {

		var rd = rand.Int63n(int64(len(g.peers))) // TODO

		peer = g.getPeer(rd)

	}

	return


}

func (g *Gossiper) checkNewPeer(peer *net.UDPAddr) {

	var senderInPeers = false

	g.lockPeers()

	for _, p := range g.peers {
		if p.String() == peer.String() {
			senderInPeers = true
		}
	}

	if !senderInPeers {
		g.peers = append(g.peers, *peer)
	}

	g.unlockPeers()

}

func (g *Gossiper) printPeers() {

	g.lockPeers()

	fmt.Print("PEERS ")

	var nbPeers = len(g.peers)
	for i, p := range g.peers {
		if i == nbPeers - 1 {
			fmt.Print(p.String())
		} else {
			fmt.Print(p.String()+",")
		}
	}

	fmt.Println()

	g.unlockPeers()

}

func make_peers(list []string) (peers []net.UDPAddr) {
	for _, e := range list {
		var addr, err = net.ResolveUDPAddr("udp4", e)
		utils.CheckError(err)
		peers = append(peers, *addr)
	}

	return
}

func (g *Gossiper) checkThreadAlive(peer *net.UDPAddr, status *primitives.StatusPacket) {

	//var waiting = g.getWaitingThread(peer.String())

	//fmt.Println("CHECK THREAD ALIVE ("+peer.String()+") : "+strconv.FormatBool(waiting))

	/*if !waiting && len(g.getChannel(peer.String())) > 0 {
		go g.waitStatusAck(peer, nil)
	}*/

	for !g.compareAndSwapThread(peer.String(), false, true) {
		time.Sleep(10 * time.Millisecond)
		fmt.Println("for loop packet lost")
	}

	g.getChannel(peer.String()) <- status

	fmt.Println("THREAD CREATED FROM CHECKTHALIVE")

	g.waitStatusAck(peer, nil)

}

func (g * Gossiper) antiEntropy() {

	var ticker = time.NewTicker(2 * time.Second)

	defer ticker.Stop()

	for {

		<- ticker.C

		if len(g.peers) > 0 {

			var peer = g.getRandomPeer()

			fmt.Println("ANTI ENTROPY")

			g.sendStatus(peer)

		}

	}

}

func (g *Gossiper) populateDSDV(rumor *primitives.RumorMessage, from *net.UDPAddr) {

	g.routingTable[rumor.Origin] = from// TODO concurrency

	fmt.Println("DSDV "+rumor.Origin+" "+from.String())

}

func (g *Gossiper) sendRouteRumorMessages(rtimer int) {

	var pckt = &primitives.ClientMessage{}
	pckt.Rumor = true

	g.receiveFromClient(pckt)

	var ticker = time.NewTicker(time.Second * time.Duration(rtimer))

	defer ticker.Stop()

	for {

		<- ticker.C

		g.receiveFromClient(pckt)

	}

}

func (g *Gossiper) listenClient(simple bool) {

	defer g.clientConn.Close()

	var buffer = make([]byte, MAX_PACKET_SIZE)

	for {

		var sz, _, errRcv = g.clientConn.ReadFromUDP(buffer)
		utils.CheckError(errRcv)

		if errRcv == nil {

			var packet = &primitives.ClientMessage{}

			var errDecode = protobuf.Decode(buffer[:sz], packet)
			utils.CheckError(errDecode)

			g.receiveFromClient(packet)

		}

	}

}

func (g *Gossiper) listenPeers(simple bool) {

	defer g.conn.Close()

	var buffer  = make([]byte, MAX_PACKET_SIZE)

	for {

		var sz, from, errRcv = g.conn.ReadFromUDP(buffer)

		utils.CheckError(errRcv)

		if errRcv == nil {

			var packet = &primitives.GossipPacket{}

			var errDecode = protobuf.Decode(buffer[:sz], packet)
			utils.CheckError(errDecode)

			g.receiveFromPeer(packet, from)
		}
	}

}

func (g *Gossiper) receiveFromClient(clientPacket *primitives.ClientMessage) {

	var packet = &primitives.GossipPacket{}

	if g.simple { // Simple

		packet.Simple = &primitives.SimpleMessage{}

		packet.Simple.OriginalName = g.name
		packet.Simple.RelayPeerAddr = g.addr.String()
		packet.Simple.Contents = clientPacket.Text

		var packetBytes, err = protobuf.Encode(packet)
		utils.CheckError(err)

		g.writeMessageClient(packet.Simple.Contents)

		for _, p := range g.peers {
			g.conn.WriteToUDP(packetBytes, &p)
		}

	} else if clientPacket.Rumor { // Rumor

		var rumor = &primitives.RumorMessage{}

		rumor.Origin = g.name
		rumor.ID = g.nextID
		rumor.Text = clientPacket.Text

		g.nextID += 1

		g.checkMsgStatus(g.name)

		g.writeMessageClient(rumor.Text)

		g.setStatusID(g.name, g.nextID)

		g.appendMsg(g.name, rumor)

		g.doRumorStep(rumor, g.getRandomPeer())

	} else if clientPacket.Private { // PrivateMessage

		privateMessage := &primitives.PrivateMessage{}
		privateMessage.Text = clientPacket.Text

		_, exists := g.routingTable[clientPacket.To]

		if !exists {
			return
		}

		privateMessage.Origin = g.name
		privateMessage.ID = 0
		privateMessage.HopLimit = HOP_LIMIT
		privateMessage.Destination = clientPacket.To

		g.sendPrivateMessage(&primitives.GossipPacket{Private:privateMessage})

	} else if clientPacket.DataRequest != nil {

		dataRequest := clientPacket.DataRequest

		_, exists := g.routingTable[dataRequest.Destination]

		if !exists {
			return
		}

		dataRequest.Origin = g.name
		dataRequest.HopLimit = HOP_LIMIT

		g.sendDataRequest(dataRequest, clientPacket.FileName, false, nil)

	} else if clientPacket.NewFile != "" {

		fmt.Println("newfile")

		g.NotifyFile(clientPacket.NewFile, "")

		fmt.Println("new file added")

	}

}

func (g *Gossiper) receiveFromPeer(packet *primitives.GossipPacket, from *net.UDPAddr) {

	fmt.Println("received pckt")

	g.checkNewPeer(from)

	if packet.Simple != nil { // Simple

		g.newMsgChannel <- g.makeServerPacket(packet)

		g.writeMessagePeer(packet, from)

		packet.Simple.RelayPeerAddr = g.addr.String()

		var packetBytes, err = protobuf.Encode(packet)
		utils.CheckError(err)

		for _, p := range g.peers {
			if p.String() != from.String() {
				g.conn.WriteToUDP(packetBytes, &p)
			}
		}

	} else if packet.Rumor != nil { // Rumor

		//fmt.Println("NEW RUMOR PACKET FROM "+from.String())

		var rumor = packet.Rumor

		if rumor.ID == g.getStatusID(rumor.Origin) {

			g.populateDSDV(rumor, from)

			if rumor.Text != "" {
				g.newMsgChannel <- g.makeServerPacket(packet)
			} else {
				g.newMsgChannel <- g.makeServerPacket(nil)
			}

			g.writeMessagePeer(packet, from)

			g.incStatusID(rumor.Origin)

			g.appendMsg(rumor.Origin, rumor)

			g.sendStatus(from)

			//fmt.Println("SENT ACK STATUS TO "+from.String())

			g.doRumorStep(rumor, g.getRandomPeer())

		} else {

			//fmt.Println("ALREADY SEEN")
			g.sendStatus(from)
			//fmt.Println("SENT ACK STATUS TO "+from.String())

		}

	} else if packet.Private != nil ||
		packet.DataRequest != nil || packet.DataReply != nil { // Private

		fmt.Println("received private msg or data req or data rep")

		isPrivate := packet.Private != nil

		destination, origin := g.getDestinationOrigin(packet)

		msg := packet.Private

		if isPrivate { g.writeMessagePeer(packet, from) }

		if destination == g.name {

			fmt.Println("received private message addressed to me")

			if isPrivate {
				if g.receivedPrivateMsgs[origin] == nil {

					g.receivedPrivateMsgs[origin] = make([]primitives.PrivateMessage, 0, MAX_MSG)

				}

				g.receivedPrivateMsgs[origin] = append(g.receivedPrivateMsgs[origin], *msg)

				fmt.Println("received private message "+packet.Private.Text+" from "+packet.Private.Origin)

				g.newMsgChannel <- g.makeServerPacket(packet)

			} else {
				if packet.DataReply != nil {

					fmt.Println("Received data reply "+hex.EncodeToString(packet.DataReply.HashValue))

					dataReply := packet.DataReply

					if g.checkHash(&dataReply.HashValue, &dataReply.Data) {

						g.dataRepliesChannel <- dataReply

					} else {
						fmt.Println("hash error")
					}


				} else { // req

					elem := g.fileElements[hex.EncodeToString(packet.DataRequest.HashValue)] // TODO concurrency

					fmt.Println("Received data request "+hex.EncodeToString(packet.DataRequest.HashValue))

					if elem != nil {

						resp := &primitives.DataReply{}

						resp.HashValue = packet.DataRequest.HashValue
						resp.Origin = g.name
						resp.Destination = packet.DataRequest.Origin
						resp.HopLimit = HOP_LIMIT

						f, err := os.OpenFile(SHARED_FILES_PREFIX+elem.Name, os.O_RDONLY, 0666)
						utils.CheckError(err)
						defer f.Close()

						buf := make([]byte, CHUNK_SIZE)

						sz, err := f.Read(buf)
						utils.CheckError(err)

						resp.Data = buf[:sz]

						fmt.Println(sz > 0)

						fmt.Println(g.checkHash(&packet.DataRequest.HashValue, &resp.Data))

						g.sendPrivateMessage(&primitives.GossipPacket{DataReply: resp})

						fmt.Println("Sent data reply : "+elem.Hash)

					}

				}
			}

		} else {

			g.sendPrivateMessage(packet)

		}

	} else { // Status

		//fmt.Println("STATUS PACKET RECEIVED FROM "+from.String())

		if g.getWaitingThread(from.String()) {
			var channel = g.getChannel(from.String())

			channel <- packet.Status
		} else {

			go g.checkThreadAlive(from, packet.Status)

		}


		//var waiting = g.getWaitingThread(from.String())

		//fmt.Println("CHANNEL HAS "+strconv.Itoa(len(channel))+" waiting msg")



		/*if !waiting {
			go g.waitStatusAck(from, nil)
		}*/

	}

}

func (g *Gossiper) notifyReply(reply *primitives.DataReply) {

	g.lockDataReplies()

	fmt.Println("reply notified")

	for _, e := range g.waitingDataReplies {

		if e.elem.Hash == hex.EncodeToString(reply.HashValue[:]) {

			fmt.Println("notified to channel")

			e.Channel <- reply

			g.unlockDataReplies()

			return

		}

	}

	g.unlockDataReplies()

}

func (g *Gossiper) listenDataReplies() {


	for {

		reply := <- g.dataRepliesChannel

		g.notifyReply(reply)

	}


}

func (g *Gossiper) waitDataReply(dataRequest *primitives.DataRequest,
	channel chan *primitives.DataReply) {

	var ticker = time.NewTicker(time.Second)

	defer ticker.Stop()

	select {

		case <- ticker.C:
			fmt.Println("Timeout from waitDataReply")
			g.sendDataRequest(dataRequest, "", true, channel)

		case reply := <- channel:
			hash := hex.EncodeToString(reply.HashValue[:])
			elem, exists := g.fileElements[hash]

			fmt.Println("waitDataReply received data")

			if !exists { // meta

				fmt.Println("meta")

				elem = g.getElem(hash)
				fmt.Println("dd")

				// construct chunks
				g.constructFileChunks(reply, elem)

				fmt.Println("chunks constructed")

			} else {

				fmt.Println("chunk : "+elem.Name)

				// write data to corresponding file
				f, err := os.OpenFile(DOWNLOADED_FILES_PREFIX+elem.Name,
					os.O_WRONLY|os.O_CREATE, 0666)
				utils.CheckError(err)

				f.Write(reply.Data)

				f.Close()

				if elem.Next == nil { // last chunk
					// add total size and set complete to true and reconstruct
					g.reconstructFile(elem.File)

					return

				}

			}

			nextHash := g.updateDataReply(elem.Hash)
			fmt.Println("Next hash : "+nextHash)

			// nextHash shouldn't be ""

			dataRequest.HopLimit = HOP_LIMIT
			v, err := hex.DecodeString(nextHash)
			utils.CheckError(err)

			dataRequest.HashValue = v

			g.sendPrivateMessage(&primitives.GossipPacket{DataRequest:dataRequest})

			ticker.Stop()

			g.waitDataReply(dataRequest, channel)


	}

}

func (g *Gossiper) reconstructFile(file *primitives.File) {

	// write the final file to the right directory, update size, set complete to true

	filePath := DOWNLOADED_FILES_PREFIX+file.Name

	f, err := os.OpenFile(filePath, // TODO directory etc
		os.O_WRONLY|os.O_CREATE, 0666)
	utils.CheckError(err)

	defer f.Close()

	size := int64(0)


	currentElem := file.First.Next

	for currentElem != nil {

		currPath := DOWNLOADED_FILES_PREFIX+currentElem.Name

		f_chunk, err := os.OpenFile(currPath,
			os.O_RDONLY, 0666)
		utils.CheckError(err)

		sz, err := io.Copy(f, f_chunk)
		utils.CheckError(err)

		size += sz

		f_chunk.Close()

		currentElem = currentElem.Next
	}

	file.Size = size

	// at the end
	file.Complete = true

}

func (g *Gossiper) constructFileChunks(reply *primitives.DataReply,
	first *primitives.FileElement) {

	file := first.File

	fmt.Println("CONSTRUCTING FILE CHUNKS")
	fmt.Println("FIRST : "+first.Hash)
	fmt.Println("NAME : "+file.Name)

	name := file.Name

	first.Name = name+"_meta"
	first.File = file

	file.First = first

	oldElem := first

	metadata := reply.Data

	f, err := os.OpenFile(DOWNLOADED_FILES_PREFIX+first.Name,
		os.O_WRONLY|os.O_CREATE, 0666)
	utils.CheckError(err)

	f.Write(reply.Data)

	f.Close()

	currentChunkIndex := 0

	file.MetaData = metadata[:]

	fmt.Println(len(metadata) > 0)
	fmt.Println(len(metadata) % 32 == 0)

	g.fileElements[first.Hash] = first

	for currentChunkIndex < len(metadata) {

		chunkHash := metadata[currentChunkIndex:currentChunkIndex + 32]

		currentElem := &primitives.FileElement{
			Hash: hex.EncodeToString(chunkHash[:]),
			Name: name+"_"+strconv.Itoa(currentChunkIndex),
			File: file,
		}

		oldElem.Next = currentElem
		oldElem = currentElem

		g.fileElements[currentElem.Hash] = currentElem

		currentChunkIndex += 32

	}

	fmt.Println("CONSTRUCTED CHUNKS :")
	curr := file.First
	for curr != nil {

		fmt.Println(curr.Hash+" -> ")

		curr = curr.Next
	}

}

func (g *Gossiper) sendDataRequest(dataRequest *primitives.DataRequest,
	fileName string,
	timeout bool,
	channel chan *primitives.DataReply) {

	hashString := hex.EncodeToString(dataRequest.HashValue[:])

	_, exists := g.fileElements[hashString]

	if exists {
		return
	}

	if !timeout {
		channel = make (chan *primitives.DataReply)
		file := &primitives.File{Name: fileName, Complete: false}
		first := &primitives.FileElement{
			Hash: hashString,
			File: file,
		}
		g.appendDataReply(
			&WaitingDataElement{
				channel,
				first,
				// TODO add methsh array
			})
	} else {

	}

	g.sendPrivateMessage(&primitives.GossipPacket{DataRequest:dataRequest})

	go g.waitDataReply(dataRequest, channel)



}


func (g *Gossiper) checkHash(hash *[]byte, data *[]byte) bool {

	h := sha256.Sum256(*data)

	return bytes.Equal(*hash, h[:])
}

func (g *Gossiper) getDestinationOrigin(packet *primitives.GossipPacket) (dest, orig string) {
	if packet.Private != nil {
		dest, orig =  packet.Private.Destination, packet.Private.Origin
	} else if packet.DataRequest != nil {
		dest, orig = packet.DataRequest.Destination, packet.DataRequest.Origin
	} else if packet.DataReply != nil {
		dest, orig = packet.DataReply.Destination, packet.DataReply.Origin
	}

	return
}

func (g *Gossiper) makeServerPacket(gossipPacket *primitives.GossipPacket) (packet *primitives.ServerPacket) {

	packet = &primitives.ServerPacket{}

	if gossipPacket != nil {
		if gossipPacket.Simple != nil {
			packet.Origin = gossipPacket.Simple.OriginalName
			packet.Content = gossipPacket.Simple.Contents
			packet.Private = false
		} else if gossipPacket.Rumor != nil {
			packet.Origin = gossipPacket.Rumor.Origin
			packet.Content = gossipPacket.Rumor.Text
			packet.Private = false
		} else if gossipPacket.Private != nil {

			packet.Origin = gossipPacket.Private.Origin
			packet.Content = gossipPacket.Private.Text
			packet.Private = true

		}
	}

	var peersString []string

	for _, peer := range g.peers {
		peersString = append(peersString, peer.String())
	}

	packet.Peers = &peersString

	var contacts []string

	for contact := range g.routingTable {
		contacts = append(contacts, contact)
	}

	packet.Contacts = &contacts

	return

}

func (g *Gossiper) doRumorStep(rumor *primitives.RumorMessage, to *net.UDPAddr) {

	if to == nil {
		return
	}

	g.sendRumorPacket(rumor.Origin, to, rumor.ID)


}

func (g *Gossiper) sendRumorPacket(from string, to *net.UDPAddr, id uint32) {

	var rumor = g.getMsg(from, id)

	var packet = primitives.GossipPacket {
		Rumor: rumor,
	}

	var packetByte, err = protobuf.Encode(&packet)
	utils.CheckError(err)

	for !g.compareAndSwapThread(to.String(), false, true) {
		time.Sleep(10 * time.Millisecond)
		//fmt.Println("for loop")
	}

	fmt.Println("THREAD CREATED FROM SENDRUMORPACKET")

	go g.waitStatusAck(to, rumor)

	g.conn.WriteToUDP(packetByte, to)

	g.notifyMongering(rumor, to)

	//fmt.Println("SENT RUMOR PACKET ("+rumor.Text+") TO "+to.String())

}

func (g *Gossiper) sendPrivateMessage(packet *primitives.GossipPacket) {

	var to string

	if packet.Private != nil {
		packet.Private.HopLimit -= 1
		to = packet.Private.Destination
	} else if packet.DataRequest != nil {
		packet.DataRequest.HopLimit -= 1
		to = packet.DataRequest.Destination
	} else if packet.DataReply != nil {
		packet.DataReply.HopLimit -= 1
		to = packet.DataReply.Destination
	}

	packetByte, err := protobuf.Encode(packet)
	utils.CheckError(err)

	g.conn.WriteToUDP(packetByte, g.routingTable[to])

}

func (g *Gossiper) waitStatusAck(from *net.UDPAddr, rumor *primitives.RumorMessage) {

	//fmt.Println("CURRENTLY WAITING FOR STATUS FROM "+from.String())

	var ticker = time.NewTicker(time.Second)

	defer ticker.Stop()

	var rd = rand.Int() % 2
	//fmt.Println("rd : "+strconv.Itoa(rd))

	var channel = g.getChannel(from.String())

	select {
	case statusRcvd := <- channel:

		fmt.Println("NO LONGER WAITING FOR STATUS FROM "+from.String())

		ticker.Stop()

		g.notifyStatus(statusRcvd, from)

		g.setWaitingThread(from.String(), false)

		//fmt.Println("RECEIVED STATUS FROM "+from.String())
		//time.Sleep(time.Second / 2)

		var synced = true

		for _, statusPeer := range statusRcvd.Want {

			var peerName = statusPeer.Identifier

			g.checkMsgStatus(peerName)

			var statusID = g.getStatusID(peerName)

			if statusID > statusPeer.NextID {

				synced = false

				/*fmt.Println(g.name+" HAS ID "+
					strconv.FormatUint(uint64(statusID), 10)+" FOR "+
					peerName+" WHEREAS "+from.String()+
					" HAS "+strconv.FormatUint(uint64(statusPeer.NextID), 10))*/



				//fmt.Println("CURRENT ID > PEER ID FOR "+peerName)
				// repeat process
				g.sendRumorPacket(peerName, from, statusPeer.NextID)
				return

			} else if statusPeer.NextID > statusID {

				//fmt.Println("PEER ID > CURRENT ID FOR "+peerName)

				synced = false

				/*fmt.Println(g.name+" HAS ID "+
					strconv.FormatUint(uint64(statusID), 10)+" FOR "+
					peerName+" WHEREAS "+from.String()+
					" HAS "+strconv.FormatUint(uint64(statusPeer.NextID), 10))*/

				// Ask for messages
				//fmt.Println("SENT CATCHUP STATUS TO "+from.String())
				g.sendStatus(from)
				return
				/*if len(g.waitingChannels[from.String()]) > 0 {
					g.waitStatusAck(from, id, rumor)
				}*/
				//return

			}
		}

		if synced {
			g.notifySync(from)
		}

		if rd == 0 && rumor != nil {
			var newPeer = g.getRandomPeerExcept(from)

			//fmt.Println("FLIP SUCCEEDED")

			if newPeer != nil {
				g.notifyCoinFlip(newPeer)
				g.doRumorStep(rumor, newPeer)
			}
		} else {
			//fmt.Println("FLIP FAILED")
			if rumor == nil {
				//fmt.Println("RUMOR WAS NIL")
			}
		}

	case <- ticker.C:
		fmt.Println("Status Timeout for "+g.addr.String()+" from "+from.String())
		//fmt.Println("TIMEOUT FROM "+from.String())
		ticker.Stop()
		g.setWaitingThread(from.String(),false)
		if rd == 0 && rumor != nil {
			var newPeer = g.getRandomPeerExcept(from)

			if newPeer != nil {
				g.notifyCoinFlip(newPeer)
			}
			g.doRumorStep(rumor, newPeer)
		} else {
			//fmt.Println("FLIP FAILED")
			if rumor == nil {
				//fmt.Println("RUMOR WAS NIL")
			}
		}
	}

	/*if len(g.waitingChannels[from.String()]) > 0 {
		g.waitStatusAck(from, id, rumor)
	}*/



}

func (g *Gossiper) makeStatus() (*[]primitives.PeerStatus) {

	g.lockStatus()

	var status = make([]primitives.PeerStatus, 0)

	for _, s := range g.status {
		status = append(status, *s)
	}

	g.unlockStatus()

	return &status

}

func (g *Gossiper) sendStatus(to *net.UDPAddr) {

	var packet = primitives.GossipPacket {
		Status: &primitives.StatusPacket {
			Want: *g.makeStatus(),
		},
	}

	var packetByte, err = protobuf.Encode(&packet)
	utils.CheckError(err)

	g.conn.WriteToUDP(packetByte, to)

	//fmt.Println("SENT STATUS TO "+to.String())

}

func (g * Gossiper) writeMessageClient(msg string) {

	if msg != "" {

		fmt.Println("CLIENT MESSAGE " + msg)

		g.printPeers()

	}

}

func (g * Gossiper) writeMessagePeer(packet *primitives.GossipPacket, from *net.UDPAddr) {

	if packet.Simple != nil {

		var msg = packet.Simple

		fmt.Println("SIMPLE MESSAGE origin " +
			msg.OriginalName + " from " + msg.RelayPeerAddr + " contents " +
			msg.Contents)

	} else if packet.Rumor != nil {

		var rumor = packet.Rumor

		if rumor.Text == "" {
			return
		}

		fmt.Println("RUMOR origin "+rumor.Origin+" from "+from.String()+" ID "+
			strconv.FormatUint(uint64(rumor.ID), 10)+
			" contents "+rumor.Text)

	} else if packet.Private != nil {

		msg := packet.Private

		fmt.Println("PRIVATE origin "+msg.Origin+" hop-limit "+strconv.FormatUint(uint64(msg.HopLimit), 10)+
			" contents "+msg.Text)

	}

	g.printPeers()

}

func (g *Gossiper) NotifyFile(name string, prepend string) {

	filePath := prepend+SHARED_FILES_PREFIX+name

	f, errFile := os.OpenFile(filePath, os.O_RDONLY, 0666)
	utils.CheckError(errFile)
	defer f.Close()

	metaName := name+"_meta"
	metaPath := prepend+SHARED_FILES_PREFIX+metaName

	f_meta, err := os.OpenFile(metaPath, os.O_RDWR|os.O_CREATE, 0666)
	utils.CheckError(err)
	defer f_meta.Close()

	counter := 0

	file := &primitives.File{Name: name, Complete: true}

	metaFileElement := &primitives.FileElement{Name: metaName, File: file}

	file.First = metaFileElement

	var oldElem = metaFileElement
	var currentElem *primitives.FileElement


	for errFile != io.EOF {

		var sz = 0

		buf := make([]byte, CHUNK_SIZE)
		sz, errFile = f.Read(buf)

		if sz == 0 && errFile == io.EOF {
			break
		}

		chunkName := name+"_"+strconv.Itoa(counter)
		chunkPath := prepend+SHARED_FILES_PREFIX+chunkName

		f_out, err := os.OpenFile(chunkPath,
			os.O_WRONLY|os.O_CREATE, 0666)
		utils.CheckError(err)

		_, err = f_out.Write(buf[:sz])
		utils.CheckError(err)

		h := sha256.Sum256(buf[:sz])
		_, err = f_meta.Write(h[:])
		utils.CheckError(err)

		f_out.Close()

		currentElem = &primitives.FileElement{
			Hash: hex.EncodeToString(h[:]),
			Name: chunkName,
			File: file,
		}

		oldElem.Next = currentElem
		oldElem = currentElem

		counter += 1
	}

	h := sha256.New()
	f_meta.Seek(0, 0)
	_, err = io.Copy(h, f_meta)
	utils.CheckError(err)

	_, err = f_meta.Read(file.MetaData)
	utils.CheckError(err)

	f_stat, err := f.Stat()
	utils.CheckError(err)

	file.Size = f_stat.Size()
	file.First.Hash = hex.EncodeToString(h.Sum(nil)[:])

	fmt.Println("New file : "+file.First.Hash)

	g.addFile(file)

}

func (g *Gossiper) addFile(file *primitives.File) {

	// TODO concurrency

	curr := file.First

	fmt.Println("FILE ADDED : ")

	for curr != nil {
		g.fileElements[curr.Hash] = curr

		fmt.Println(curr.Hash+" -> ")

		curr = curr.Next
	}

}

func (g *Gossiper) notifyMongering(rumor *primitives.RumorMessage, to *net.UDPAddr) {

	fmt.Println("MONGERING with "+to.String())

}

func (g *Gossiper) notifyStatus(statusPacket *primitives.StatusPacket, from *net.UDPAddr) {

	fmt.Print("STATUS from "+from.String()+" ")

	fmt.Println(statusPacket == nil)

	for _, status := range statusPacket.Want {

		fmt.Print("peer "+status.Identifier+" nextID "+
			strconv.FormatUint(uint64(status.NextID), 10)+" ")

	}

	fmt.Println()

	fmt.Print("MY STATUS ")

	for _, s := range g.status {

		fmt.Print("peer "+s.Identifier+" nextID "+
			strconv.FormatUint(uint64(s.NextID), 10)+" ")

	}

	fmt.Println()

}

func (g *Gossiper) notifyCoinFlip(to *net.UDPAddr) {

	fmt.Println("FLIPPED COIN sending rumor to "+to.String())

}

func (g *Gossiper) notifySync(with *net.UDPAddr) {

	fmt.Println("IN SYNC WITH "+with.String())

}


