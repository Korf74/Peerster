package gossip

import (
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
const MAX_PEERS = 2048
const MAX_PACKET_SIZE = 2048
const MAX_MSG = 2048
const MAX_CHANNEL_BUFFER = 2048
const HOP_LIMIT = 10

/* TODO IDEA : a function that receives channels and statuses and pair them, if new status
** TODO wihout chennel -> create
 */

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
	nextID uint32
	newMsgChannel chan *primitives.ServerPacket
	muxPeers sync.Mutex // TODO REALLY NEED IT ? THREAD BY PEER
	muxStatus sync.Mutex
	muxChannels sync.Mutex // TODO SAME
	muxThreads sync.Mutex
	muxMsgs sync.Mutex

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
		nextID: 1,
		newMsgChannel: channel,
		muxPeers: sync.Mutex{},
		muxStatus: sync.Mutex{},
		muxThreads: sync.Mutex{},
		muxChannels: sync.Mutex{},
		muxMsgs: sync.Mutex{},
	}

	channel <- g.makeServerPacket(nil)

	go g.listenPeers(simple)
	go g.antiEntropy()
	go g.listenClient(simple)
	if rtimer > 0 { go g.sendRouteRumorMessages(rtimer) }

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
	pckt.Private = false

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

	} else if !clientPacket.Private { // Rumor

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

	} else { // PrivateMessage

		privateMessage := &primitives.PrivateMessage{}

		to := clientPacket.To

		_, exists := g.routingTable[to]

		if !exists {
			return
		}

		privateMessage.Origin = g.name
		privateMessage.ID = 0
		privateMessage.Text = clientPacket.Text
		privateMessage.Destination = to
		privateMessage.HopLimit = HOP_LIMIT


		g.sendPrivateMessage(privateMessage, to)

	}

}

func (g *Gossiper) receiveFromPeer(packet *primitives.GossipPacket, from *net.UDPAddr) {

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

	} else if packet.Private != nil { // Private

		msg := packet.Private

		g.writeMessagePeer(packet, from)

		if msg.Destination == g.name {

			origin := msg.Origin

			if g.receivedPrivateMsgs[origin] == nil {

				g.receivedPrivateMsgs[origin] = make([]primitives.PrivateMessage, 0, MAX_MSG)

			}

			g.receivedPrivateMsgs[origin] = append(g.receivedPrivateMsgs[origin], *msg)

			g.newMsgChannel <- g.makeServerPacket(packet)

		} else {

			msg.HopLimit -= 1

			g.sendPrivateMessage(msg, msg.Destination)

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

	fmt.Println("CONTACTS : "+strconv.Itoa(len(g.routingTable)))

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

func (g *Gossiper) sendPrivateMessage(msg *primitives.PrivateMessage, to string) {

	packet := primitives.GossipPacket{
		Private: msg,
	}

	packetByte, err := protobuf.Encode(&packet)
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

func (g *Gossiper) NotifyFile(name string) {

	f, err := os.OpenFile(name, os.O_RDONLY, 0666)
	utils.CheckError(err)
	defer f.Close()

	counter := 0

	for err == nil {

		f_out, errFile := os.OpenFile(name+"_"+strconv.Itoa(counter),
			os.O_WRONLY|os.O_CREATE, 0666)
		utils.CheckError(errFile)

		_, err = io.CopyN(f_out, f,8000)

		f_out.Close()

		counter += 1
	}

	// TODO

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


