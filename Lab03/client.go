package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var (
	ipAddress            string
	port                 int
	peerAddress          string
	peerPort             int
	stabilizeTime        int
	fixFingersTime       int
	checkPredecessorTime int
	numberOfSuccessor    int
	idClient             string

	client *ChordClient
)

func lookup(file string) {
	fmt.Println("LOOKUP FILE: ", file)

	host := client.lookup(file)

	for i := 0; i < len(host); i++ {
		succ := client.findSuccessor(host[i].Id)
		printFinger(succ)
	}

	for i := 0; i < len(host); i++ {

		encryptedFile, fileExists := client.AskGetFile(host[i].IpAddress, file)
		if fileExists {
			data := decryptFile(encryptedFile, "123")
			fmt.Print(string(data))
			break
		} else {
			fmt.Println("File Does not Exist")
		}
	}
}

func storeFile(file string) {
	fileName := getFileName(file)

	fmt.Println("STORING FILE: ", fileName)

	data, err := os.ReadFile(file)

	encryptedData := encyptFile(data, "123")

	if err != nil {
		println("Error reading the file aborting")
		return
	}

	client.StoreFile(fileName, encryptedData)
}

func main() {
	fmt.Println("START CHORD")

	err := initFlag()
	if err != nil {
		log.Fatal(err)
		os.Exit(-1)
	}

	// start client
	client = initializeClient()

	redirectStandardError(hex.EncodeToString(client.identifier[:]) + ".log")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.Split(scanner.Text(), " ")
		if len(input) > 2 {
			fmt.Println("Wrong Number of Args to COMMAND")
			os.Exit(-1)
		}

		switch input[0] {
		case "Lookup":
			file := input[1]
			lookup(file)
		case "StoreFile":
			file := input[1]
			storeFile(file)

		case "PrintState":
			fmt.Printf("My Address: %s:%s\n", client.myIp, client.myPort)
			fmt.Println("------FINGER TABLE-----")
			client.fingerLock.Lock()
			for i := 0; i < len(client.fingerTable); i++ {
				printFinger(client.fingerTable[i])
			}
			client.fingerLock.Unlock()
			fmt.Println("------FINGER TABLE-----")

			fmt.Println("------SUCCESSOR LIST-----")
			client.successorLock.Lock()
			for i := 0; i < len(client.successor); i++ {
				printFinger(client.successor[i])
			}
			client.successorLock.Unlock()
			fmt.Println("------SUCCESSOR LIST-----")
			client.predecessorLock.Lock()
			if client.predecessor != nil {
				fmt.Println("Predecessor: ")
				printFinger(*client.predecessor)
			}
			client.predecessorLock.Unlock()

		default:
			fmt.Println("WRONG COMMAND")
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println(err)
	}

	fmt.Println("END CHORD")
}

func printFinger(finger finger) {
	fmt.Printf("ID: %x\n", finger.Id)
	fmt.Printf("Address: %s\n", finger.IpAddress)
}

func initFlag() error {
	flag.StringVar(&ipAddress, "a", "", "help message")
	flag.IntVar(&port, "p", 0, "aaa")
	flag.StringVar(&peerAddress, "ja", "", "help message")
	flag.IntVar(&peerPort, "jp", 0, "aaa")
	flag.IntVar(&stabilizeTime, "ts", 0, "aaa")
	flag.IntVar(&fixFingersTime, "tff", 0, "aaa")
	flag.IntVar(&checkPredecessorTime, "tcp", 0, "aaa")
	flag.IntVar(&numberOfSuccessor, "r", 0, "aaa")
	flag.StringVar(&idClient, "i", "", "help message")

	flag.Parse()

	if (peerAddress != "" && peerPort == 0) || (peerAddress == "" && peerPort != 0) {
		return errors.New("ERROR IN FLAG")
	}

	if ipAddress == "" || port == 0 {
		return errors.New("ERROR IN FLAG")
	}

	fmt.Println("------FLAGS-------------")
	fmt.Println("ipAdress:", ipAddress)
	fmt.Println("port:", port)
	fmt.Println("peerAddress:", peerAddress)
	fmt.Println("peerPort:", peerPort)
	fmt.Println("stabilizeTime:", stabilizeTime)
	fmt.Println("fixFingersTime:", fixFingersTime)
	fmt.Println("checkPredecessorTime:", checkPredecessorTime)
	fmt.Println("numberOfSuccessor:", numberOfSuccessor)
	fmt.Println("idClient:", idClient)
	fmt.Println("tail:", flag.Args())
	fmt.Println("------FLAGS-------------")
	return nil
}

func initializeClient() *ChordClient {

	if peerAddress != "" && peerPort != 0 {
		// CONNECT TO RING
		return join(
			ipAddress,
			numberOfSuccessor,
			port,
			idClient,
			stabilizeTime,
			fixFingersTime,
			checkPredecessorTime,
			peerAddress,
			peerPort)
	}

	if peerAddress == "" && peerPort == 0 {
		// CREATE RING
		return create(ipAddress, numberOfSuccessor, port, idClient, stabilizeTime, fixFingersTime, checkPredecessorTime)
	}
	return nil
}

func getFileName(path string) string {
	elements := strings.Split(path, "/")

	return elements[len(elements)-1]
}

func encyptFile(file []byte, keyPhrase string) []byte {
	hash := md5.Sum([]byte(keyPhrase))
	aesBlock, err := aes.NewCipher(hash[:])
	if err != nil {
		fmt.Println(err)
	}

	gcmInstance, err := cipher.NewGCM(aesBlock)
	if err != nil {
		fmt.Println(err)
	}
	nonce := make([]byte, gcmInstance.NonceSize())
	_, _ = io.ReadFull(rand.Reader, nonce)

	return gcmInstance.Seal(nonce, nonce, file, nil)
}

func decryptFile(file []byte, keyPhrase string) []byte {
	hash := md5.Sum([]byte(keyPhrase))
	aesBlock, err := aes.NewCipher(hash[:])
	if err != nil {
		fmt.Println(err)
	}

	gcmInstance, err := cipher.NewGCM(aesBlock)
	if err != nil {
		fmt.Println(err)
	}
	nonceSize := gcmInstance.NonceSize()
	nonce, encryptedFile := file[:nonceSize], file[nonceSize:]
	decryptedFile, err := gcmInstance.Open(nil, nonce, encryptedFile, nil)
	if err != nil {
		fmt.Println(err)
	}
	return decryptedFile
}

func redirectStandardError(file string) {
	newStdErr, err := os.Create(file)
	if err != nil {
		log.Println("Unable to create log file")
		return
	}
	log.SetOutput(newStdErr)
}
