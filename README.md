# Peerster

This repository holds the GO project created for the Master course "Decentralized Systems Engineering" at EPFL (Ecole Polytechnique Federale de Lausanne).

There are several chunks to this project corresponding to different deadlines.

This repository is mostly used by me to give visibility to this project as I do for similar ones, and for the TAs to grade it.

The main goal is to create a decentralized chat system called Peerster, as a peer to peer application using concepts seen in lectures such as rumor mongering, anti-entropy or cryptography.

The nodes communcating with eachother are called *gossipers* and client communicate to their gossipers messages to be relayed to other gossipers. Gossipers use UDP as opposed to TCP.

## First Homework

There were three important parts for this first deadline.

### Simple messages

This very first part focused on being able to send and receive very simple message and simply broadcast them to your whole list of peers. Receiving a message from an unknown peer makes your gossiper add it to its list.

To use this part :

First do

`go build`

Gossiper : 

`./Peerster -UIPort=CLIENT-PORT -gossipAddr=GOSSIP-IP -name=NAME -peers=LIST-OF-PEERS -simple`

Client : 

`client/client -UIPort=CLIENT-PORT -msg=YOUR-MESSAGE`

The *-simple* flag forces the gossiper to use simple messages.

### Rumor mongering and anti-entropy

For this part, we introduced the concepts of rumor-mongering and anti-entropy to the code. Simply put, for rumor-mongering gossipers will send messages at random, and continue to do so randomly. Anti-Entropy sends a random message every *T* amount of time. Having both of these protocols ensures messages are indeed received, and that the expected time for a node to receive a message is not too high.

To use this part :

First do

`go build`

Gossiper : 

`./Peerster -UIPort=CLIENT-PORT -gossipAddr=GOSSIP-IP -name=NAME -peers=LIST-OF-PEERS`

Client : 

`client/client -UIPort=CLIENT-PORT -msg=YOUR-MESSAGE`

Notice the absence of the *-simple* flag.

### A simple GUI

For this last part we had to implement a simple GUI tool on a static web page using html/javascript.

Its main purpose is to simulate the client part, it should display current known nodes (gossipers), a chat box with received messages and a way to send messages, and it should have a way to add new peers.

The GUI should be communicating with a server listening on the port *8080*.

To use the gui simply go the server directory and type 

`go build`

`./server`

Then go to your favorite web browser, [connect to the server](localhost:8080) (at *127.0.0.1:8080*). You will be prompted an address for your gossiper (as for the *gossipAddr* flag in previous parts), but not the port that will be handled internally. 

You will see a chat box with your current address and port, and all the required functionalities.

Adding a peer with a bad address should raise an error. Adding a peer with a correct address will add it to the list of peers but might not appear immediatly on the GUI list so please don't spam the *ADD PEER* button.

The GUI is simplistic and to update sends a GET query to the server every 5 seconds. 

The way I coded my server permits you to open the page several times to create several gossipers (which is convenient for local tests without having to type the whole command every time).

However please be careful that since the page queries the server periodically, you shouldn't have an old GUI page open while rebooting the server (it might cause errors), simply close the page if you want to reboot the server.

## Second Homework

### Routing

A routing table was added with a regular poll permetting to populate a DSDV table correpsponding to the possible contacts. This permits to have point to point messaging with private messages and file sharing.

### Private messages

To use private messages, on the GUI you can select via radio buttons the contact you want to communicate with, this will show the private chatroom corresponding to this contact.

With the CLI use the *-dest* option.

### File sharing

To use file sharing you first need to have files available in the directory *_SharedFiles*, either do it manually or use the uploading option from the GUI.

When indexing a file to the gossiper (done withing the upload with the GUI, or with the *-file* option with the CLI), you can get the metadata corresponding to the metafile. This hash can be used to query files to other peers if they have it.

Using the GUI, fill the form on the top of the page with the hash of the metafile, the contact to ask to and the name of the file. Using the CLI, use *-file* and *-dest*.

For now, there is a bug for which it won't work for too big files but it works well for smaller files.


