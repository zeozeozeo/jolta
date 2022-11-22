# Jolta

This is an event-based networking library for Go, based on [KCP](https://github.com/xtaci/kcp-go) and [Smux](https://github.com/xtaci/smux).

# Features

-   Secure: all of the data is encrypted by default, and there's no way to disable it.
-   Small latency: the KCP protocol is [designed for small latencies](https://raw.githubusercontent.com/skywind3000/kcp/master/images/spatialos-50.png).
-   Small: the entire library is ~650 lines of code.

# Examples

## (Server) Start a server

```go
package main

import "github.com/zeozeozeo/jolta"

func main() {
    // server address, password, salt
    server := jolta.NewServer("127.0.0.1:7145", []byte("test password"), []byte("test salt"))

    // listen (this is blocking)
	if err := server.Listen(); err != nil {
		panic(err)
	}
}
```

## (Client) Connect to a server

```go
package main

import "github.com/zeozeozeo/jolta"

func main() {
    // server address, password and salt
    // of the server you want to connect to
    client := jolta.NewClient("127.0.0.1:7145", []byte("test password"), []byte("test salt"))

    // connect to the server (this is blocking)
    if err := client.Connect(); err != nil {
		panic(err)
	}
}
```

## (Client) Send and recieve messages to/from other clients

```go
func main() {
    // client := jolta.NewClient(...)

    client.OnConnect(func(client *jolta.Client) {
        // send a message to all connected
        // clients, the event name can be
        // any string
		client.SendAll("any event name", []byte("hello from client!"))

        // get all clients connected to
        // the server
        clients, err := client.GetClients()
        if err != nil || len(clients) == 0 {
            return
        }

        // send a message to some client
        // (clients are represented with
        // ID's, 1 being the client that
        // connected first)
        client.SendTo("any event name", clients[0], []byte("hi client!"))
	})

    // recieve messages from other clients
	client.On("any event name", func(client *jolta.Client, senderId uint32, data []byte) {
		// do anything you want...
        fmt.Printf("client %d says: %s\n", senderId, string(data))
	})

    // client.Connect()...
}
```
