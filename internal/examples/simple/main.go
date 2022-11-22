package main

import (
	"fmt"
	"time"

	"github.com/zeozeozeo/jolta"
)

const (
	ADDRESS string = "127.0.0.1:7145"
)

var (
	PASSWORD []byte = []byte("test password")
	SALT     []byte = []byte("test salt")
)

func main() {
	go server()
	time.Sleep(1 * time.Second) // wait until the server is started

	// make a new client
	client := jolta.NewClient(ADDRESS, PASSWORD, SALT)

	client.OnConnect(func(client *jolta.Client) {
		fmt.Println("client connected!")

		// send an event to all clients (since we only have one client,
		// noone will recieve it)
		client.SendAll("any event name", []byte("hello from client!"))
	})

	// an example of handling an event from other clients:
	client.On("another event name", func(client *jolta.Client, senderId uint32, data []byte) {
		// do anything you want...
	})

	if err := client.Connect(); err != nil {
		panic(err)
	}
}

func server() {
	server := jolta.NewServer(ADDRESS, PASSWORD, SALT)

	server.OnStart(func(server *jolta.Server) {
		fmt.Println("the server has started!")
	})

	if err := server.Listen(); err != nil {
		panic(err)
	}
}
