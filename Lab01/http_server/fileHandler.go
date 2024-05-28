package main

import (
	"errors"
	"os"
)

const (
	BASE_PATH             = "public" // The folder where the content of the http will reside
	ERROR_PATH_INVALID    = "path is not valid"
	ERROR_PATH_NOT_FOUND  = "path not found"
	ERROR_CREATING_FILE   = "failed creating file"
	ERROR_TRUNCATING_FILE = "failed truncating file"
	ERROR_WRITING_FILE    = "failed to write the file"
)

// check if the required path does not exit the root path, should be called before any read/write on files to avoid reading or writing on private files
//
// path is the path to check
//
// returns whether the path is safe or not
func checkPath(path string) bool {
	var depth = 0
	for i := 0; i < len(path)-4; i++ {
		if path[i] == '/' && i != 0 {
			depth++
		}
		if path[i] == '/' && path[i+1] == '.' && path[i+2] == '.' && path[i+3] == '/' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return true
}

// reads a binary file and returns the byte stream, hoping that it fits in memory
//
// path is the path of the file to read
//
// return the content of the file or an error if there is one
func readBinaryFile(path string) (*[]byte, error) {
	if !checkPath(path) {
		return nil, errors.New(ERROR_PATH_INVALID) // should cause a 401 FORBIDDEN
	}
	data, err := os.ReadFile(BASE_PATH + path)
	if err != nil {
		return nil, errors.New(ERROR_PATH_NOT_FOUND) // should cause a 404 NOT FOUND
	}

	return &data, nil
}

// writes the binary file to disk
//
// path	is the path to write to the file to write and data is the data to write to the file
//
// returns error if any
func writeBinaryFile(path string, data []byte) error {
	if !checkPath(path) {
		return errors.New(ERROR_PATH_INVALID) // should cause a 401 FORBIDDEN
	}

	file, err := os.Create(BASE_PATH + path)
	if err != nil {
		println(err.Error())
		return errors.New(ERROR_CREATING_FILE) // should cause a 500 INTERNAL SERVER ERROR
	}

	defer file.Close() // close the file

	err = file.Truncate(0)
	if err != nil {
		return errors.New(ERROR_TRUNCATING_FILE) // should cause a 500 INTERNAL SERVER ERROR
	}

	_, err = file.Write(data)
	if err != nil {
		return errors.New(ERROR_WRITING_FILE) // should cause a 500 INTERNAL SERVER ERROR
	}

	return nil
}
