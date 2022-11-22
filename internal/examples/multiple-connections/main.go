package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/zeozeozeo/jolta"
)

const (
	ADDRESS string = "127.0.0.1:7145"
)

var (
	PASSWORD  []byte = []byte("test password")
	SALT      []byte = []byte("test salt")
	isRunning bool
)

func main() {
	fmt.Println("starting server")
	go server()
	time.Sleep(1 * time.Second) // wait until the server is started
	fmt.Println("starting clients")

	// create 10 clients
	for i := 0; i < 10; i++ {
		go client()
	}

	for isRunning {
		time.Sleep(100 * time.Millisecond)
	}
}

func client() {
	client := jolta.NewClient(ADDRESS, PASSWORD, SALT)

	client.OnConnect(func(client *jolta.Client) {
		fmt.Printf("client %d connected!\n", client.ID())

		go func() {
			for {
				// get all clients connected to the server
				clients, err := client.GetClients()
				if err != nil || len(clients) == 0 {
					continue
				}

				// choose a random client
				randomClient := clients[rand.Intn(len(clients))]

				// send a message to the client
				message := fmt.Sprintf("client %d says hello to client %d!", client.ID(), randomClient)
				client.SendTo("some event name", randomClient, []byte(message))

				// wait 1-10 seconds
				time.Sleep(time.Duration(rand.Intn(10-1)+1) * time.Second)
			}
		}()
	})

	// add a handler to an event
	client.On("some event name", func(client *jolta.Client, senderId uint32, data []byte) {
		// print the message we got
		fmt.Println(string(data))
	})

	// start the client
	if err := client.Connect(); err != nil {
		panic(err)
	}
}

func server() {
	isRunning = true

	// create and start the server
	server := jolta.NewServer(ADDRESS, PASSWORD, SALT)

	server.OnStart(func(server *jolta.Server) {
		fmt.Println("the server has started!")
	})

	server.OnConnect(func(conn *jolta.Connection) {
		fmt.Printf("server: client %d sent a message\n", conn.ID())
	})

	if err := server.Listen(); err != nil {
		panic(err)
	}

	isRunning = false
}
