package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sync/semaphore"
)

const (
	MAX_SOCKETS_WORKERS = 10
)

func main() {
	var (
		port string
	)
	// Used to control the total number of open sockets
	sem := semaphore.NewWeighted(MAX_SOCKETS_WORKERS)
	ctx := context.TODO()

	// Select the correct port
	if len(os.Args) != 2 {
		println("No port was specified, listening to 8080")
		port = "8080"
	} else {
		port = os.Args[1]
		fmt.Printf("Listening on port %s", port)
	}

	var listenAddress = ":" + port

	// Listen to the selected port
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		println("Unable to bind the given port")
		println(err)
		os.Exit(1)
	}

	defer ln.Close() // Ensure that the socket is correctly closed at the termination of the program

	// Handle the connections accepted
	for {
		conn, err := ln.Accept()
		if err != nil {
			println("There was an error listening!")
			println(err)
			if errors.Is(err, net.ErrClosed) {
				os.Exit(1)
			}
		} else {
			sem.Acquire(ctx, 1)
			go handleConnection(conn, sem)
		}
	}
}
