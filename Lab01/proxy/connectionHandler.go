package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/sync/semaphore"
)

const (
	ERROR_BAD_REQUEST = "unable to contact server"
)

// handle the request if it is a Get request then close the connection with the client and free one space in the semaphore
//
// connection is the connection on which the communication occurs and sem is the semaphore with the weight of the maximum amount of connections that the proxy should be able to handle at the same time
func handleConnection(connection net.Conn, sem *semaphore.Weighted) {
	defer sem.Release(1)
	defer connection.Close()

	println("Received a connection from " + connection.RemoteAddr().String() + " at " + time.Now().GoString())
	var reader = bufio.NewReader(connection)
	request, err := http.ReadRequest(reader)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}

	defer request.Body.Close() // it is important to close it at the end

	switch request.Method {
	case "GET":
		println("Request for " + request.Host + request.URL.EscapedPath())
		handleGet(request, connection)
	default:
		body := io.NopCloser(bytes.NewReader(make([]byte, 0)))
		header := make(http.Header, 0)
		header.Add("connection", "close")
		response := http.Response{
			Status:        "Not Implemented",
			StatusCode:    501,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          body,
			ContentLength: 0,
			Request:       request,
			Header:        header,
		}
		response.Write(connection)
	}
	println("end of connection")
}

// get the string with the full address to reach the server and the desired file
//
// address is either "host" or "host:port"
//
// returns host:port if exists or uses default port 80 and return host:80 (standard value for server)
func getFullAdressString(address string) string {
	for i := 0; i < len(address); i++ {
		if address[i] == ':' {
			return address
		}
	}
	return address + ":80"
}

// handle the Get request received by transmitting the request to the server, receiving its answer and transmitting it back to the client
//
// request is the http.Request received in the Get request on the connection link described by connection
func handleGet(request *http.Request, connection net.Conn) {
	serverConn, err := net.Dial("tcp", getFullAdressString(request.URL.Host))
	if err != nil {
		writeErrorResponse(errors.New(ERROR_BAD_REQUEST), connection, request)
		return
	}

	defer serverConn.Close()

	// https://localhost:8080/test.txt?123=321 -> EscapedPath -> /test.txt
	path := request.URL.EscapedPath() //returns the path (without the host)
	serverReq, err := http.NewRequest(request.Method, path, request.Body)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}
	println("Contacting the server")
	serverReq.Write(serverConn)

	var reader = bufio.NewReader(serverConn)
	answer, err := http.ReadResponse(reader, serverReq)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}
	println("No error in receiving")
	answer.Write(connection)
	println("Sending the answer to the client")
}

// print the description of the error
//
// err is the error catched and conn is the connection on which the error occured
func writeErrorResponse(err error, conn net.Conn, request *http.Request) {
	resp := getErrorResponse(err, request)
	resp.Write(conn)
}

// get the packet in the correct data structure (with a header and a body) to use for the specified error
//
// err is the error for which to generate the response
//
// returns the complete response
func getErrorResponse(err error, request *http.Request) http.Response {
	var status string
	var statusCode int

	switch err.Error() {
	case ERROR_BAD_REQUEST:
		status = "Bad Gateway"
		statusCode = 502
	default:
		status = "Bad Request"
		statusCode = 400
	}

	body := io.NopCloser(bytes.NewReader([]byte(err.Error())))

	header := make(http.Header, 0)
	header.Add("connection", "close")
	header.Add("content-type", "text/plain")

	return http.Response{
		Status:        status,
		StatusCode:    statusCode,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          body,
		ContentLength: 0,
		Request:       request,
		Header:        header,
	}
}
