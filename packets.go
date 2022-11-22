package jolta

type Packet uint8

const (
	PACKET_GETID      Packet = 0 // This packet is sent by the client when it's connected, the server responds with the client's ID.
	PACKET_SENDTO     Packet = 1 // The client tells the server to send a message to a specified user ID
	PACKET_SENDALL    Packet = 2 // The client tells the server to send a message to all clients
	PACKET_MESSAGE    Packet = 3 // The server tells the client that someone sent it a new message
	PACKET_GETCLIENTS Packet = 4 // The client asks the server to list all clients
)
