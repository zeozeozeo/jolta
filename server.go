package jolta

import (
	"crypto/sha1"
	"io"
	"time"

	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
)

type Server struct {
	Address      string
	Password     []byte
	Salt         []byte
	DataShards   int // default: 10
	ParityShards int // default: 3
	Connections  []*Connection
	KeyIter      int           // default: 1024
	KeyLen       int           // default: 32
	Timeout      time.Duration // Timeout for accepting streams. 0 disables the timeout.

	isAlive      bool
	onStart      func(server *Server)
	onConnect    func(conn *Connection)
	onDisconnect func(conn *Connection)
	onMessage    func(conn *Connection, eventName string, senderId uint32, data []byte)
}

type Connection struct {
	session *smux.Session
	server  *Server
	id      uint32
}

// NewServer creates a new server context.
func NewServer(address string, password, salt []byte) *Server {
	server := &Server{
		Address:      address,
		Password:     password,
		Salt:         salt,
		DataShards:   10,
		ParityShards: 3,
		KeyIter:      1024,
		KeyLen:       32,
		isAlive:      true,
	}

	return server
}

// Listen starts the server and listens for connections.
func (server *Server) Listen() error {
	key := pbkdf2.Key(server.Password, server.Salt, server.KeyIter, server.KeyLen, sha1.New)
	block, err := kcp.NewAESBlockCrypt(key)
	if err != nil {
		return err
	}

	listener, err := kcp.ListenWithOptions(
		server.Address,
		block,
		server.DataShards,
		server.ParityShards,
	)

	if err != nil {
		return err
	}

	if server.onStart != nil {
		server.onStart(server)
	}

	for server.isAlive {
		// wait for a connection
		sess, err := listener.AcceptKCP()
		if err != nil {
			return err
		}

		// setup server side of smux
		session, err := smux.Server(sess, nil)
		if err != nil {
			return err
		}

		// handle incoming connection
		go server.handleConnection(session)
	}
	return nil
}

func (server *Server) getNextFreeClientId() uint32 {
	id := uint32(1) // all valid id's start with 1
	for i := 0; i < len(server.Connections); i++ {
		if server.Connections[i].id == id {
			id++
			i = 0
		}
	}
	return id
}

func (server *Server) handleConnection(session *smux.Session) {
	conn := &Connection{
		session: session,
		server:  server,
		id:      server.getNextFreeClientId(),
	}
	server.Connections = append(server.Connections, conn)

	conn.startLoop(session)
}

// Stop disconnects all clients and stops the server.
func (server *Server) Stop() {
	for _, c := range server.Connections {
		c.session.Close()
	}

	server.Connections = nil
	server.isAlive = false
}

func (conn *Connection) startLoop(session *smux.Session) {
	for {
		if session.IsClosed() {
			conn.Disconnect()
			return
		}

		// wait for an incoming stream
		if conn.server.Timeout != 0 {
			session.SetDeadline(time.Now().Add(conn.server.Timeout))
		}
		stream, err := session.AcceptStream()
		if err != nil {
			conn.Disconnect()
			return
		}

		// TODO: this returns an error
		conn.handleStream(stream)
	}
}

// Disconnect disconnects the client from the server.
func (conn *Connection) Disconnect() {
	if conn.server.onDisconnect != nil {
		conn.server.onDisconnect(conn)
	}

	conn.session.Close()
	for idx, serverConn := range conn.server.Connections {
		if serverConn == conn {
			removeIndex(conn.server.Connections, idx)
			break
		}
	}
}

func (conn *Connection) handleStream(stream *smux.Stream) error {
	defer stream.Close()

	// read packet type
	packetType, err := readPacket(stream)
	if err != nil {
		return err
	}

	switch packetType {
	case PACKET_GETID:
		prevId := conn.id
		err = writeUint32(stream, conn.id)
		if prevId == 0 && conn.server.onConnect != nil {
			conn.server.onConnect(conn)
		}
	case PACKET_SENDTO:
		err = conn.handleSendTo(stream)
	case PACKET_SENDALL:
		err = conn.handleSendAll(stream)
	case PACKET_GETCLIENTS:
		// clients length
		if err := writeUint32(stream, uint32(len(conn.server.Connections))); err != nil {
			return err
		}

		// clients
		for _, serverConn := range conn.server.Connections {
			if err := writeUint32(stream, serverConn.id); err != nil {
				return err
			}
		}
	}

	return err
}

func (conn *Connection) handleSendTo(r io.Reader) error {
	// event name
	eventName, err := readEventName(r)
	if err != nil {
		return err
	}

	// client id
	clientId, err := readUint32(r)
	if err != nil {
		return err
	}

	// data
	data, err := readData(r)
	if err != nil {
		return err
	}

	// find a user with that ID
	for _, serverConn := range conn.server.Connections {
		if serverConn.id == clientId {
			serverConn.Send(eventName, conn.id, data)
			break
		}
	}

	return nil
}

func (conn *Connection) handleSendAll(r io.Reader) error {
	// event name
	eventName, err := readEventName(r)
	if err != nil {
		return err
	}

	// data
	data, err := readData(r)
	if err != nil {
		return err
	}

	// send the message to all users
	for _, serverConn := range conn.server.Connections {
		serverConn.Send(eventName, conn.id, data)
	}

	return nil
}

// Send sends a message to the client.
func (conn *Connection) Send(eventName string, senderId uint32, data []byte) error {
	stream, err := conn.session.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	// PACKET_MESSAGE, event name length, event name, sender ID, data length, data
	if err := writePacket(stream, PACKET_MESSAGE); err != nil {
		return err
	}

	if err := writeEventName(stream, eventName); err != nil {
		return err
	}
	if err := writeUint32(stream, senderId); err != nil {
		return err
	}
	if err := writeData(stream, data); err != nil {
		return err
	}

	if conn.server.onMessage != nil {
		conn.server.onMessage(conn, eventName, senderId, data)
	}

	return nil
}

// ID returns the client ID.
func (conn *Connection) ID() uint32 {
	return conn.id
}

// OnStart sets the function that is called when the server is started.
func (server *Server) OnStart(on func(server *Server)) {
	server.onStart = on
}

// OnConnect sets the function that is called when a client is connected.
func (server *Server) OnConnect(on func(conn *Connection)) {
	server.onConnect = on
}

// OnDisconnect sets the function that is called when a client is connected.
func (server *Server) OnDisconnect(on func(conn *Connection)) {
	server.onDisconnect = on
}

// OnMessage sets the function that is called when a client recieves a message.
func (server *Server) OnMessage(on func(conn *Connection, eventName string, senderId uint32, data []byte)) {
	server.onMessage = on
}
