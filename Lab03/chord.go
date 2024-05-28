package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type finger struct {
	Id        [20]byte
	IpAddress string //"ip:port"
}

type ChordClient struct {
	identifier      [20]byte
	fingerLock      sync.Mutex
	fingerTable     []finger
	successorLock   sync.Mutex
	successor       []finger
	predecessorLock sync.Mutex
	predecessor     *finger
	myIp            string
	myPort          string
	numSucc         int
	next            int
}

// start the server and the RPC connection
func (client *ChordClient) server() {
	rpc.Register(client)
	rpc.HandleHTTP()

	l, e := net.Listen("tcp", fmt.Sprintf("%s:%s", client.myIp, client.myPort))
	if e != nil {
		log.Fatal(e.Error())
		os.Exit(1)
	}

	go http.Serve(l, nil)
}

// create a new chord ring
// return the first member of this chord ring
func create(IP string, numSucc int, port int, ID string, timeStabilisation int, timeFixFinger int, timeCheckPred int) *ChordClient {
	println("CREATE")

	result := new(ChordClient)

	result.myIp = IP
	result.myPort = fmt.Sprint(port)

	if ID != "" {
		tmp, err := hex.DecodeString(ID)
		if err != nil {
			log.Fatal(err.Error())
		}

		// reduce the id length if needed and put it in a list
		for len(tmp) < 20 {
			tmp = append(make([]byte, 1), tmp...)
		}

		for i := 0; i < 20; i++ {
			result.identifier[i] = tmp[i]
		}
	} else {
		// generate an identifier if it is not given
		result.identifier = sha1.Sum(fmt.Append(make([]byte, 0), result.myIp, ":", result.myPort))
	}

	result.successor = make([]finger, numSucc)
	result.fingerTable = make([]finger, 160)

	// initialize our first successor as ourself as the server is alone in the network now
	result.successor[0] = finger{result.identifier, result.myIp + ":" + result.myPort}
	result.predecessor = nil
	result.numSucc = numSucc
	result.next = -1

	if os.Mkdir(hex.EncodeToString(result.identifier[:]), 0777) != nil {
		log.Println("Unable to create folder, fatal")
		//		os.Exit(-1)
	}

	// launch several go routines to run the updating functions concurrently
	result.server()
	go result.runStabilize(timeStabilisation)
	go result.runFixFingers(timeFixFinger)
	go result.runCheckPred(timeCheckPred)
	println("CREATED")

	return result
}

// run the stabilize function for some time on the client
// take interval as an argument, to know often the function should be runned
func (client *ChordClient) runStabilize(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)

	client.stabilize()
	for range ticker.C {
		log.Println("Begin Stabilize")
		start := time.Now()
		client.stabilize()
		duration := time.Since(start)
		log.Printf("[Verbose] Stabilized in %f\n", duration.Seconds())
	}
}

// run the fixFinger function for some time on the client
// take interval as an argument, to know often the function should be runned
func (client *ChordClient) runFixFingers(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)

	for i := 0; i < 160; i++ {
		client.fixFinger()
	}
	for range ticker.C {
		log.Println("Begin Fix Finger")
		start := time.Now()
		client.fixFinger()
		duration := time.Since(start)
		log.Printf("[Verbose] Fixed fingers in %f\n", duration.Seconds())
	}
}

// run the check_predecessor function for some time on the client
// take interval as an argument, to know often the function should be runned
func (client *ChordClient) runCheckPred(interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)

	for range ticker.C {
		start := time.Now()
		client.check_predecessor()
		duration := time.Since(start)
		log.Printf("[Verbose] Checked predecessor in %f\n", duration.Seconds())
	}
}

// create a new chord ring using the create function then add it to an existing ring using a device within this ring
// take IPClient and portClient as arguments to know which client to contact to join the ring
func join(IP string, numSucc int, port int, ID string, timeStabilisation int, timeFixFinger int, timeCheckPred int, IPClient string, portClient int) *ChordClient {
	result := create(IP, numSucc, port, ID, timeStabilisation, timeFixFinger, timeCheckPred)

	// ask the device from the ring that we know about our successor in the ring
	clientIknow := IPClient + ":" + fmt.Sprint(portClient)
	fingerOfSuccessor, err := result.askFindSuccessor(clientIknow, result.identifier)
	if !err {
		// EXPLODE?
		log.Fatal("FATAL: Unable to connect to the ring.")
	}
	fmt.Printf("---------JOIN SUCCESSOR: %s -------------", fingerOfSuccessor.IpAddress)
	result.successor[0] = fingerOfSuccessor

	err = result.askUpdatePredecessor(result.successor[0].IpAddress)
	if !err {
		log.Fatal("SOMETHING WRONG")
	}

	return result
}

// find the successor of the device whos identifier is given in argument
// argument: hash as the identifier of the device for which we are looking for the successor
// return the finger of the successor of the device given in argument
func (client *ChordClient) findSuccessor(hash [20]byte) finger {
	client.successorLock.Lock()
	successor := client.successor[0]
	client.successorLock.Unlock()

	// DO NOT SPLIT THE IFS that is the easiest way to get something that does not work
	// Spend the time and understand why they are that way !!!

	if !keyInBetweenRightInclusive(client.identifier, successor.Id, hash) {
		// if the hash is not between the client and the successor, we are calling recursively
		// for the successor of the hash on its closest preceding node from the finger table of client
		closestPredAddress := client.findClosestPrecedingNode(hash)
		var success bool
		successor, success = client.askFindSuccessor(closestPredAddress, hash)
		if !success {
			log.Println("Failed to contact successor retrying in 10ms")

			time.Sleep(10 * time.Millisecond)

			client.successorLock.Lock()
			client.checkSuccessor()
			client.updateSuccessorList()
			client.successorLock.Unlock()

			return client.findSuccessor(hash)
		}
	}
	return successor
}

// ACQUIRE SUCCESSOR LOCK BEFORE CALLING THIS FUNCTION
func (client *ChordClient) checkSuccessor() {
	// start by finding the first successor of client that client can still reach
	successor := client.successor[0]
	firstWorking := 0
	alive := client.askHealthNode(successor.IpAddress)
	if !alive {
		//time.Sleep(10 * time.Millisecond)
		for i := 0; i < len(client.successor); i++ {
			successor = client.successor[i]
			alive = successor.IpAddress != "" && client.askHealthNode(successor.IpAddress)
			if !alive {
				firstWorking++
			} else {
				break
			}
		}
		// then update the successor list
		if firstWorking == len(client.successor) {
			// if no successor is reachable anymore, client is alone in the ring so it is its own successor
			var itself finger
			itself.Id = client.identifier
			itself.IpAddress = client.myIp + ":" + client.myPort
			log.Println("Not found any valid succ")
			client.successor[0] = itself
		} else {
			log.Println("Found valid succ")
			client.successor[0] = client.successor[firstWorking]
		}
		client.updateSuccessorList()
	}
	log.Println("Finished check successor")
}

// ACQUIRE successorLock before calling
// updates the successor lists
func (client *ChordClient) updateSuccessorList() {
	log.Printf("Updating successor List, asking, %s\n", client.successor[0].IpAddress)

	newSuccList := client.AskSuccList(client.successor[0].IpAddress)

	if newSuccList != nil { // when is this not the case?
		for i := 1; i < len(newSuccList); i++ {
			client.successor[i] = newSuccList[i-1]
		}
	} else {
		log.Println("SuccessorList is nil trying to fix it")
		client.checkSuccessor()
	}
	log.Println("Finished updating successor List")
}

// find the higher node from a finger table and successor list that is before a hash number
// argument: hash as the identifier of the device for which we are looking for the preceding node
// return the address and port of the closest preceding node of hash in the client's finger table
func (client *ChordClient) findClosestPrecedingNode(hash [20]byte) string {
	// FIND GREATEST HASH in finger table LESS THAN HASH parameter

	var maxFingerTable finger
	var maxSuccessorList finger

	client.fingerLock.Lock()
	// Search max in Finger Table
	for i := client.next - 1; i >= 0; i-- {
		if client.fingerTable[i].IpAddress != "" {
			if keyInBetweenExclusive(client.identifier, hash, client.fingerTable[i].Id) {
				maxFingerTable = client.fingerTable[i]
				break // the break is necessary because I want to exist at when I find the first
			}
		}
	}
	client.fingerLock.Unlock()

	client.successorLock.Lock()
	// Search max in Successor List
	for i := client.numSucc - 1; i >= 0; i-- {
		if client.successor[i].IpAddress != "" {
			if keyInBetweenExclusive(client.identifier, hash, client.successor[i].Id) {
				maxSuccessorList = client.successor[i]
				break
			}
		}
	}
	client.successorLock.Unlock()

	if maxFingerTable.IpAddress == "" && maxSuccessorList.IpAddress == "" {
		return client.myIp + ":" + client.myPort
	} else {
		if maxFingerTable.IpAddress == "" {
			return maxSuccessorList.IpAddress
		}
		if maxSuccessorList.IpAddress == "" {
			return maxFingerTable.IpAddress
		}

		if compareKeys(maxFingerTable.Id, maxSuccessorList.Id) {
			return maxFingerTable.IpAddress
		} else {
			return maxSuccessorList.IpAddress
		}
	}
}

// actualize the finger table and successor table of the client by making sure that the client
// is still the predecessor of its successor
//
// return nothing
func (client *ChordClient) stabilize() {
	client.successorLock.Lock()
	client.checkSuccessor()
	client.updateSuccessorList()
	successor := client.successor[0]
	client.successorLock.Unlock()

	// Get predecessor of successor of client
	currentPredecessor, err := client.askGetPredecessor(successor.IpAddress)
	if !err {
		err = client.askUpdatePredecessor(successor.IpAddress)
		if !err {
			log.Println("unable to set myself as predecessor of my successor (intended if I am my successor)")
		}
		return
	}

	if keyInBetweenExclusive(client.identifier, successor.Id, currentPredecessor.Id) {
		// Acquire the lock only if necessary to avoid excessive synchronization
		// Check that the condition is still true after acquiring the lock
		// Necessary because the successor is a copy
		client.successorLock.Lock()
		if keyInBetweenExclusive(client.identifier, client.successor[0].Id, currentPredecessor.Id) &&
			client.askHealthNode(currentPredecessor.IpAddress) { // here we need to check that it is also alive in case
			client.successor[0] = currentPredecessor
			successor = currentPredecessor
			client.updateSuccessorList()
		}
		client.successorLock.Unlock()
	}

	// If client is current predecessor, do nothing
	// else do RPC call to Successor to update his predecessor with client
	err = client.askUpdatePredecessor(successor.IpAddress)
	if !err {
		log.Println("Unable to update our successor that we are their predecessor aborting stabilize for now")
		return
	}

	directory := hex.EncodeToString(client.identifier[:]) + "/"
	// Open the directory
	outputDirRead, _ := os.Open(directory)
	// Call ReadDir to get all files
	outputDirFiles, _ := outputDirRead.ReadDir(0)

	// Loop over files
	for outputIndex := range outputDirFiles {
		outputFileHere := outputDirFiles[outputIndex]
		// Get name of file
		outputNameHere := outputFileHere.Name()

		log.Println("Checking file " + outputNameHere)

		// Get the list of the nodes where the file should be
		dest := client.lookup(outputNameHere)
		correct := false
		for i := 0; i < len(dest); i++ {
			// if the nodes where the file should be are the same as client, the file is at the right place
			if sameKey(client.identifier, dest[i].Id) {
				correct = true
				break // I don't care anymore
			}
		}

		fileToGet := fmt.Sprintf("%s/%s", hex.EncodeToString(client.identifier[:]), outputNameHere)
		data, err := os.ReadFile(fileToGet)
		if err != nil {
			log.Fatal("File system error")
		}

		// this should be done regardless of wheather I am supposed to store that file or not for faster duplication
		for i := 0; i < len(dest); i++ {
			log.Printf("Asking %s if they have the file.\n", dest[i].IpAddress)
			_, exist := client.AskGetFile(dest[i].IpAddress, outputNameHere)
			if !exist {
				log.Printf("They do not, sending.\n")
				client.AskStoreFile(dest[i].IpAddress, outputNameHere, data)
			} else {
				log.Printf("They do, continuing.\n")
			}
		}

		if !correct {
			// if the nodes where the file should be are different than client, the file should be moved
			e := os.Remove(fileToGet)
			if e != nil {
				log.Fatal(e)
			}
		}
	}

	// os.Readdir maybe get the list of files in our directory
	// using lookup check that we are still the correct successor
	// if we are not we should use the storefile to send the file to the correct node
	// 			if we store on multiple nodes we should send the data again to all the other nodes
	// 			BIG PROBLEM set the timing for the stabilize to a big number like 15s
	//			Before sending to the other check if they have the file already, if they do do not send anything
	// Delete the file that sent to us
}

// actualise the predecessor of client if maybePredecessor is indeed between our previous predecessor and client
// argument: maybePredecessor which is a pointer to the finger of the node asking to update client's predecessor
// returns true if maybePredecessor is indeed the new predecessor of client now, else returns false
func (client *ChordClient) notify(maybePredecessor *finger) bool {

	if sameKey(client.identifier, maybePredecessor.Id) {
		return false
	}

	result := false

	client.predecessorLock.Lock()
	if client.predecessor == nil || sameKey(maybePredecessor.Id, client.predecessor.Id) || keyInBetweenExclusive(client.predecessor.Id, client.identifier, maybePredecessor.Id) {
		log.Print("in notify")
		log.Println(maybePredecessor.IpAddress)
		client.predecessor = maybePredecessor
		result = true
	}

	client.predecessorLock.Unlock()

	return result
}

// update the finger table
func (client *ChordClient) fixFinger() {
	client.next = client.next + 1
	if client.next > 159 {
		client.next = 0
	}

	newHash := keyToLook(client.next, client.identifier)
	tmpResult := client.findSuccessor(newHash)
	client.fingerLock.Lock()
	client.fingerTable[client.next] = tmpResult
	client.fingerLock.Unlock()
}

// update client's predecessor
func (client *ChordClient) check_predecessor() {
	client.predecessorLock.Lock()
	if client.predecessor != nil && !client.askHealthNode(client.predecessor.IpAddress) {
		client.predecessor = nil
	}
	client.predecessorLock.Unlock()
}

// find the nodes's hash where the file should be stored
// argument: fileName is the string containing the name of the file to handle
// return a tab containing the fingers of where the file should be
func (client *ChordClient) lookup(fileName string) []finger {
	nbFiles := 3
	tabHashOfFile := make([]finger, nbFiles*2)

	for i := 0; i < nbFiles; i++ {
		// calulate the hash of the name of the file
		fileNameHash := sha1.Sum([]byte(fileName))
		nextFingerFile := keyToLook(i*100, fileNameHash)

		// find the successor of these values, where the file should be
		node := client.findSuccessor(nextFingerFile)
		tabHashOfFile[i*2] = node

		successors := client.AskSuccList(node.IpAddress)

		tabHashOfFile[i*2+1] = successors[0]
	}
	return tabHashOfFile
}

// store the file in the client
// arguments: key and data are the name of the file and its content
// no return
func (client *ChordClient) StoreFile(key string, data []byte) {
	// store the file in all the items in the list
	whoShouldHold := client.lookup(key)

	for i := 0; i < len(whoShouldHold); i++ {
		client.AskStoreFile(whoShouldHold[i].IpAddress, key, data)
	}
}
