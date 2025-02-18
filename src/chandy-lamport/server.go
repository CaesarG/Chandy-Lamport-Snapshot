package chandy_lamport

import (
	"log"
)

// void implements set to avoid extra memo cost
type void struct{}

var TRUE void

// The main participant of the distributed snapshot protocol.
// Servers exchange token messages and marker messages among each other.
// Token messages represent the transfer of tokens from one server to another.
// Marker messages represent the progress of the snapshot process. The bulk of
// the distributed protocol is implemented in `HandlePacket` and `StartSnapshot`.
type Server struct {
	Id            string
	Tokens        int
	sim           *Simulator
	outboundLinks map[string]*Link // key = link.dest
	inboundLinks  map[string]*Link // key = link.src
	totReceived   map[int]int
	Received      map[int]map[string]void
}

// A unidirectional communication channel between two servers
// Each link contains an event queue (as opposed to a packet queue)
type Link struct {
	src    string
	dest   string
	events *Queue
}

func NewServer(id string, tokens int, sim *Simulator) *Server {
	return &Server{
		id,
		tokens,
		sim,
		make(map[string]*Link),
		make(map[string]*Link),
		make(map[int]int),
		make(map[int]map[string]void),
	}
}

// Add a unidirectional link to the destination server
func (server *Server) AddOutboundLink(dest *Server) {
	if server == dest {
		return
	}
	l := Link{server.Id, dest.Id, NewQueue()}
	server.outboundLinks[dest.Id] = &l
	dest.inboundLinks[server.Id] = &l
}

// Send a message on all of the server's outbound links
func (server *Server) SendToNeighbors(message interface{}) {
	for _, serverId := range getSortedKeys(server.outboundLinks) {
		link := server.outboundLinks[serverId]
		server.sim.logger.RecordEvent(
			server,
			SentMessageEvent{server.Id, link.dest, message})
		link.events.Push(SendMessageEvent{
			server.Id,
			link.dest,
			message,
			server.sim.GetReceiveTime()})
	}
}

// Send a number of tokens to a neighbor attached to this server
func (server *Server) SendTokens(numTokens int, dest string) {
	if server.Tokens < numTokens {
		log.Fatalf("Server %v attempted to send %v tokens when it only has %v\n",
			server.Id, numTokens, server.Tokens)
	}
	message := TokenMessage{numTokens}
	server.sim.logger.RecordEvent(server, SentMessageEvent{server.Id, dest, message})
	// Update local state before sending the tokens
	server.Tokens -= numTokens
	link, ok := server.outboundLinks[dest]
	if !ok {
		log.Fatalf("Unknown dest ID %v from server %v\n", dest, server.Id)
	}
	link.events.Push(SendMessageEvent{
		server.Id,
		dest,
		message,
		server.sim.GetReceiveTime()})
}

// Callback for when a message is received on this server.
// When the snapshot algorithm completes on this server, this function
// should notify the simulator by calling `sim.NotifySnapshotComplete`.
func (server *Server) HandlePacket(src string, message interface{}) {
	// TODO: IMPLEMENT ME
	switch msg := message.(type) {
	case TokenMessage:
		server.Tokens += msg.numTokens
		for snapshotId, received := range server.Received {
			if _, ok := received[src]; !ok {
				snapMessages := server.sim.snapMessages[snapshotId]
				server.sim.snapMessages[snapshotId] = append(snapMessages, &SnapshotMessage{src, server.Id, msg})
			}
		}
	case MarkerMessage:
		snapshotId := msg.snapshotId
		_, exist := server.totReceived[snapshotId]
		if !exist {
			server.totReceived[snapshotId] = 1
			server.Received[snapshotId] = make(map[string]void)
			server.Received[snapshotId][src] = TRUE
			// store own status and send marker to neighbors
			if _, ok := server.sim.snapTokens[snapshotId]; !ok {
				server.sim.snapTokens[snapshotId] = make(map[string]int)
			}
			snapTokens := server.sim.snapTokens[snapshotId]
			if _, ok := snapTokens[server.Id]; !ok {
				snapTokens[server.Id] = server.Tokens
				server.SendToNeighbors(MarkerMessage{snapshotId})
			}
		} else if _, ok := server.Received[snapshotId][src]; !ok {
			server.totReceived[snapshotId]++
			server.Received[snapshotId][src] = TRUE

		} else {
			return
		}
		// server has received markers from all incoming channels
		if server.totReceived[snapshotId] == len(server.inboundLinks) {
			server.sim.NotifySnapshotComplete(server.Id, snapshotId)
		}
	}
}

// Start the chandy-lamport snapshot algorithm on this server.
// This should be called only once per server.
func (server *Server) StartSnapshot(snapshotId int) {
	// TODO: IMPLEMENT ME
	// store own status and send marker to neighbors
	if _, ok := server.sim.snapTokens[snapshotId]; !ok {
		server.sim.snapTokens[snapshotId] = make(map[string]int)
	}
	snapTokens := server.sim.snapTokens[snapshotId]
	if _, ok := snapTokens[server.Id]; !ok {
		snapTokens[server.Id] = server.Tokens
		server.SendToNeighbors(MarkerMessage{snapshotId})
	}
	server.Received[snapshotId] = make(map[string]void)
}
