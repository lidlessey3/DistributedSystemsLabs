package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/rpc"
	"os"
)

// This file will contain all the rpc struct definitions and all rpc calls definitions

//***************************************
//****************STRUCTS****************
//***************************************

type FindSuccessorArgs struct {
	HashToLook [20]byte // the successor of this hash to look for
	MyId       [20]byte // my id
}

type FindSuccessorRet struct {
	SuccessorId [20]byte
	SuccessorAd string // the struct of the successor
}

type GetPredecessorRet struct {
	PredecessorId    [20]byte
	PredecessorAd    string // the predecessor of the node
	IHavePredecessor bool
}

type GetPredecessorArgs struct {
	Id [20]byte // the id of the asking one
}

type UpdatePredecessorArgs struct {
	PredecessorId [20]byte
	PredecessorAd string // the new predecessor to put
}

type UpdatePredecessorRet struct {
	Done             bool // should always be true
	OldPredecessorId [20]byte
	OldPredecessorAd string
}

type HealthRet struct {
	MyId [20]byte // my id, just used for pings
}

type HealthArgs struct {
	MyId [20]byte // same as above
}

type SuccListArgs struct {
	MyId [20]byte
}

type SuccListRet struct {
	SuccList []finger
}

type StoreFileArgs struct {
	Key  string
	Data []byte
}

type StoreFileRet struct {
	Done bool
}

type GetFileArgs struct {
	FileName string
}

type GetFileRet struct {
	Data       []byte
	FileExists bool
}

//***************************************
//*****************CALLS*****************
//***************************************

// Handlers

// execute the findSuccessor function if a RPC request is received
// arguments: Args containing the identifier of the device looking for its successor
//
//	Ret containing the identifier and the IP address of the successor if found
//
// return nil as findSuccessor already handle errors
func (client *ChordClient) ReqFindSuccessor(Args *FindSuccessorArgs, Ret *FindSuccessorRet) error {
	if sameKey(Args.MyId, client.identifier) {
		log.Fatal("I should not be here what happened?")
	}
	result := client.findSuccessor(Args.HashToLook)
	Ret.SuccessorAd = result.IpAddress
	Ret.SuccessorId = result.Id
	return nil
}

func (client *ChordClient) ReqGetPredecessor(Args *GetPredecessorArgs, Ret *GetPredecessorRet) error {

	client.predecessorLock.Lock()
	if client.predecessor == nil {
		client.predecessorLock.Unlock()
		Ret.IHavePredecessor = false
		return nil // SHOULD BE AN ERROR
	}
	Ret.PredecessorAd = client.predecessor.IpAddress
	Ret.PredecessorId = client.predecessor.Id
	client.predecessorLock.Unlock()
	Ret.IHavePredecessor = true
	return nil
}

func (client *ChordClient) ReqUpdatePredecessor(Args *UpdatePredecessorArgs, Ret *UpdatePredecessorRet) error {
	pred := new(finger)

	pred.Id = Args.PredecessorId
	pred.IpAddress = Args.PredecessorAd

	client.predecessorLock.Lock()
	if client.predecessor != nil {
		Ret.OldPredecessorId = pred.Id
		Ret.OldPredecessorAd = pred.IpAddress
	}
	client.predecessorLock.Unlock()

	//println("====")
	//println(Ret.Done)

	// if !sameKey(client.identifier, pred.Id) {
	// 	Ret.Done = client.notify(pred)
	// }

	Ret.Done = client.notify(pred)

	//println("==11=")
	//println(Ret.Done)
	return nil
}

func (client *ChordClient) ReqHealth(Args *HealthArgs, Ret *HealthRet) error {
	Ret.MyId = client.identifier
	return nil
}

func (client *ChordClient) ReqSuccList(Args *SuccListArgs, Ret *SuccListRet) error {
	if !sameKey(Args.MyId, client.identifier) {
		client.successorLock.Lock()
	}
	Ret.SuccList = append(Ret.SuccList, client.successor...)
	if !sameKey(Args.MyId, client.identifier) {
		client.successorLock.Unlock()
	}

	return nil
}

func (client *ChordClient) ReqStoreFile(Args *StoreFileArgs, Ret *StoreFileRet) error {
	file, err := os.Create(fmt.Sprintf("%s/%s", hex.EncodeToString(client.identifier[:]), Args.Key))
	if err != nil {
		Ret.Done = false
		return err
	}
	Ret.Done = true

	file.Write(Args.Data)
	file.Close()
	return nil
}

func (client *ChordClient) ReqGetFile(Args *GetFileArgs, Ret *GetFileRet) error {
	fileToGet := fmt.Sprintf("%s/%s", hex.EncodeToString(client.identifier[:]), Args.FileName)

	var data []byte
	var fileExists bool

	if _, err := os.Stat(fileToGet); err == nil {
		fileData, err := os.ReadFile(fileToGet)
		if err != nil {
			return err
		}
		data = fileData
		fileExists = true
	} else {
		data = nil
		fileExists = false
	}

	Ret.Data = data
	Ret.FileExists = fileExists
	return nil
}

// Callers

// call the ReqFindSuccessor function to make a RPC request to another device about our successor
// arguments: destAddress for the IP address and port of the device we want to reach,
//
//	hash for the identifier of the device asking for this demand
//
// return a finger of the successor of the device asking in the ring and
//
//	a boolean indicating if the function went well and a successor has been found
func (client *ChordClient) askFindSuccessor(destAddress string, hash [20]byte) (finger, bool) {
	var args FindSuccessorArgs
	var ret FindSuccessorRet

	args.HashToLook = hash
	args.MyId = client.identifier

	if !call(destAddress, "ChordClient.ReqFindSuccessor", &args, &ret) {
		return finger{[20]byte{}, ""}, false
	}

	var result finger

	result.Id = ret.SuccessorId
	result.IpAddress = ret.SuccessorAd

	return result, true
}

func (client *ChordClient) askGetPredecessor(successorIp string) (finger, bool) {
	var ret GetPredecessorRet
	var args GetPredecessorArgs

	args.Id = client.identifier

	if !call(successorIp, "ChordClient.ReqGetPredecessor", &args, &ret) {
		return finger{[20]byte{}, ""}, false
	}

	var result finger

	result.Id = ret.PredecessorId
	result.IpAddress = ret.PredecessorAd

	return result, ret.IHavePredecessor
}

func (client *ChordClient) askUpdatePredecessor(successorIp string) bool {
	var args UpdatePredecessorArgs
	var ret UpdatePredecessorRet

	args.PredecessorId = client.identifier
	args.PredecessorAd = client.myIp + ":" + client.myPort

	resultOfCall := call(successorIp, "ChordClient.ReqUpdatePredecessor", &args, &ret)
	if !resultOfCall || !ret.Done {
		return false
	}

	return true
}

func (client *ChordClient) askHealthNode(ip string) bool {
	var ret HealthRet
	var args HealthArgs

	args.MyId = client.identifier

	return call(ip, "ChordClient.ReqHealth", &args, &ret)
}

func (client *ChordClient) AskSuccList(ip string) []finger {
	var ret SuccListRet
	var args SuccListArgs
	args.MyId = client.identifier

	if !call(ip, "ChordClient.ReqSuccList", &args, &ret) {
		return nil
	}

	return ret.SuccList
}

func (client *ChordClient) AskStoreFile(ip string, key string, data []byte) {
	var args StoreFileArgs
	var ret StoreFileRet

	args.Data = data
	args.Key = key

	if !call(ip, "ChordClient.ReqStoreFile", &args, &ret) {
		println("I should not be here")
	}
}

func (client *ChordClient) AskGetFile(ipAddr string, fileName string) ([]byte, bool) {
	var args GetFileArgs
	var ret GetFileRet

	args.FileName = fileName

	if !call(ipAddr, "ChordClient.ReqGetFile", &args, &ret) {
		return nil, false
	}

	return ret.Data, ret.FileExists
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(destinationAddress, rpcname string, args interface{}, reply interface{}) bool {
	var c *rpc.Client
	var err error
	
	c, err = rpc.DialHTTP("tcp", destinationAddress)

	if err != nil {
		// c should not be closed in this case as it may lead to termination for some reason
		log.Print("dialing:", err)
		return false
	}

	err = c.Call(rpcname, args, reply)
	c.Close()
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
