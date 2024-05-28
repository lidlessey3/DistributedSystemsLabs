package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

var (
	fileFormats = map[string]string{
		"html": "text/html",
		"css":  "text/css",
		"txt":  "text/plain",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"gif":  "image/gif",
	}
)

// handles the incoming connection and closes it afterward, freeing one space on the semaphore
//
// connection is the socket to the open connection and sem is the semaphore used to limit the maximum number of concurrent routines running
func handleConnection(connection net.Conn, sem *semaphore.Weighted) {
	defer connection.Close() // ensure the connection is closed
	defer sem.Release(1)     // ensure the semaphore is released

	println("Received a connection from " + connection.RemoteAddr().String() + " at " + time.Now().GoString())
	var reader = bufio.NewReader(connection)
	request, err := http.ReadRequest(reader)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}

	defer request.Body.Close() // it is important to close the reader at the end, to free up resources I guess

	switch request.Method {
	case "GET":
		println("Request for " + request.URL.EscapedPath())
		handleGet(request, connection)
	case "POST":
		println("Request for " + request.URL.EscapedPath())
		handlePost(request, connection)
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

// handle the Get request received by transmitting the request to the server, receiving its answer and transmitting it back to the client
//
// request is the http.Request received in the Get request on the connection link described by connection
func handleGet(request *http.Request, connection net.Conn) {
	fileType, err := getFileType(request.URL.EscapedPath())
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}
	data, err := readBinaryFile(request.URL.EscapedPath())
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}
	reader := bytes.NewReader(*data)
	readerCloser := io.NopCloser(reader)

	response := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          readerCloser,
		ContentLength: int64(len(*data)),
		Request:       request,
		Header:        make(http.Header, 0),
	}
	response.Header.Add("content-type", fileFormats[fileType])
	response.Header.Add("connection", "close")

	// Another possibility is to first serializes the response into a buffer and then write the buffer to the socket
	/*var buf bytes.Buffer
	response.Write(&buf)
	connection.Write(buf.Bytes())*/

	response.Write(connection) // writing to the socket the correct answer
}

// handle the Post request received by creating or opening the file mentioned in the request and send a confirmation message back to the client
//
// request is the http.Request received in the Post request on the connection link described by connection
func handlePost(request *http.Request, connection net.Conn) {
	typeOfFile, err := getFileType(request.URL.EscapedPath())
	if err != nil || fileFormats[typeOfFile] != request.Header.Get("Content-Type") {
		if err == nil {
			err = errors.New("filetype mismatch")
		}
		writeErrorResponse(err, connection, request)
		return
	}

	data, err := io.ReadAll(request.Body)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}

	err = writeBinaryFile(request.URL.EscapedPath(), data)
	if err != nil {
		writeErrorResponse(err, connection, request)
		return
	}

	body := io.NopCloser(bytes.NewReader(make([]byte, 0)))

	header := make(http.Header, 0)
	header.Add("connection", "close")

	response := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
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

// print the description of the error
//
// err is the error catched and conn is the connection on which the error occured
func writeErrorResponse(err error, connection net.Conn, request *http.Request) {
	resp := getErrorResponse(err, request)
	resp.Write(connection)
}

// returns the response data structure to use for the specified error
//
// err is the error for which to generate the response
//
// returns the complete response
func getErrorResponse(err error, request *http.Request) http.Response {
	var status string
	var statusCode int
	switch err.Error() {
	case ERROR_PATH_INVALID:
		status = "Forbidden"
		statusCode = 403
	case ERROR_PATH_NOT_FOUND:
		status = "Not Found"
		statusCode = 404
	case ERROR_CREATING_FILE:
		status = "Internal Server Error"
		statusCode = 500
	case ERROR_WRITING_FILE:
		status = "Internal Server Error"
		statusCode = 500
	case ERROR_TRUNCATING_FILE:
		status = "Internal Server Error"
		statusCode = 500
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

// returns the extension of the file if it is among the supported ones
//
// path is the string to parse
//
// returns the extension or an error
func getFileType(path string) (string, error) {
	var builder strings.Builder
	for i := len(path) - 1; i >= 0 && path[i] != '.'; i-- {
		builder.WriteByte(path[i])
	}
	var res = Reverse(builder.String())

	_, exist := fileFormats[res]
	if exist {
		return res, nil
	} else {
		return "", errors.New("file type not supported")
	}
}

// reverses the given string
//
// s is the string to reverse
//
// returns the inverted string
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
