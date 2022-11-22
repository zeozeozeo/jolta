package jolta

import (
	"crypto/sha1"
	"errors"
	"io"
	"time"

	"github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
	"golang.org/x/crypto/pbkdf2"
)

type Client struct {
	Address      string
	Password     []byte
	Salt         []byte
	DataShards   int // default: 10.
	ParityShards int // default: 3.
	KeyIter      int // default: 1024.
	KeyLen       int // default: 32.
	Events       []*Event
	Timeout      time.Duration // Timeout for accepting streams. 0 disables the timeout.

	isAlive      bool
	session      *smux.Session
	id           uint32
	onConnect    func(client *Client)
	onDisconnect func(client *Client)
}

type Event struct {
	Func func(client *Client, senderId uint32, data []byte)
	Name string
}

var (
	ErrDisconnected error = errors.New("jolta client: disconnected")
)

// NewClient creates a new client.
func NewClient(address string, password, salt []byte) *Client {
	client := &Client{
		Address:      address,
		Password:     password,
		Salt:         salt,
		DataShards:   10,
		ParityShards: 3,
		KeyIter:      1024,
		KeyLen:       32,
		isAlive:      true,
	}
	return client
}

// Connect connects the client to the server.
func (client *Client) Connect() error {
	key := pbkdf2.Key(client.Password, client.Salt, client.KeyIter, client.KeyLen, sha1.New)
	block, err := kcp.NewAESBlockCrypt(key)
	if err != nil {
		return err
	}

	conn, err := kcp.DialWithOptions(
		client.Address,
		block,
		client.DataShards,
		client.ParityShards,
	)
	if err != nil {
		return err
	}

	// setup smux
	session, err := smux.Client(conn, nil)
	if err != nil {
		return err
	}
	client.session = session

	client.sendConnectionPacket(session)
	return client.startLoop(session)
}

func (client *Client) sendConnectionPacket(session *smux.Session) error {
	stream, err := session.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := writePacket(stream, PACKET_GETID); err != nil {
		return err
	}

	// the server should respond with the client id
	id, err := readUint32(stream)
	if err != nil {
		return err
	}
	client.id = id

	if client.onConnect != nil {
		client.onConnect(client)
	}
	return nil
}

func (client *Client) startLoop(session *smux.Session) error {
	defer client.Disconnect()
	for client.isAlive {
		if session.IsClosed() {
			client.Disconnect()
			return ErrDisconnected
		}

		// wait for an incoming stream
		if client.Timeout != 0 {
			session.SetDeadline(time.Now().Add(client.Timeout))
		}
		stream, err := session.AcceptStream()
		if err != nil {
			return err
		}

		if err := client.handleStream(stream); err != nil {
			return err
		}
	}

	return nil
}

// Disconnect disconnects the client from the server.
func (client *Client) Disconnect() {
	if client.onDisconnect != nil {
		client.onDisconnect(client)
	}

	if client.session == nil {
		return
	}

	client.session.Close()
}

func (client *Client) handleStream(stream *smux.Stream) error {
	defer stream.Close()
	// read packet type
	packetType, err := readPacket(stream)
	if err != nil {
		client.Disconnect()
		return err
	}

	switch packetType {
	case PACKET_MESSAGE:
		client.handleMessage(stream)
	}

	return nil
}

// OnDisconnect sets the function that is called when the client is connected.
func (client *Client) OnConnect(on func(client *Client)) {
	client.onConnect = on
}

// OnDisconnect sets the function that is called before the client is disconnected.
func (client *Client) OnDisconnect(on func(client *Client)) {
	client.onDisconnect = on
}

// SendTo sends a message to a client with that ID.
func (client *Client) SendTo(eventName string, id uint32, data []byte) error {
	stream, err := client.session.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := writePacket(stream, PACKET_SENDTO); err != nil {
		return err
	}

	// the server expects an event name length, event name string,
	// client id, compression, data length, data
	if err := writeEventName(stream, eventName); err != nil {
		return err
	}

	// client id
	if err := writeUint32(stream, id); err != nil {
		return err
	}

	// data
	return writeData(stream, data)
}

// SendAll sends a message to all clients.
func (client *Client) SendAll(eventName string, data []byte) error {
	stream, err := client.session.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := writePacket(stream, PACKET_SENDTO); err != nil {
		return err
	}

	// the server expects an event name length, event name string,
	// data length, data
	if err := writeEventName(stream, eventName); err != nil {
		return err
	}

	// data
	return writeData(stream, data)
}

// GetClients returns all client ID's that are connected to the server.
func (client *Client) GetClients() ([]uint32, error) {
	stream, err := client.session.OpenStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	if err := writePacket(stream, PACKET_GETCLIENTS); err != nil {
		return nil, err
	}

	// clients length, clients
	clientsLen, err := readUint32(stream)
	if err != nil {
		return nil, err
	}

	clients := []uint32{}
	for i := uint32(0); i < clientsLen; i++ {
		id, err := readUint32(stream)
		if err != nil {
			return nil, err
		}
		clients = append(clients, id)
	}

	return clients, nil
}

func (client *Client) handleMessage(r io.Reader) error {
	eventName, err := readEventName(r)
	if err != nil {
		return err
	}

	senderId, err := readUint32(r)
	if err != nil {
		return err
	}

	data, err := readData(r)
	if err != nil {
		return err
	}

	// find handlers for this event
	for _, ev := range client.Events {
		if ev.Func != nil && (ev.Name == eventName || ev.Name == "") {
			ev.Func(client, senderId, data)
		}
	}

	return nil
}

// On adds a hander to an event name. Empty event names will be called on any event.
func (client *Client) On(eventName string, on func(client *Client, senderId uint32, data []byte)) {
	client.Events = append(client.Events, &Event{
		Func: on,
		Name: eventName,
	})
}

// Returns the client ID. An ID of 0 means that the client is not connected yet.
func (client *Client) ID() uint32 {
	return client.id
}
