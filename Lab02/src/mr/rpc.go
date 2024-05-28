package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

// Add your RPC definitions here.
type AskTaskArgs struct {
	Id int // The id of the worker that is asking, -1 if not assigned yet
}

type AskTaskReply struct {
	Id       int      // The id of the worker that is calling
	TaskType int      // The type of task, 1 if map 2 if reduce
	TaskId   int      // The id of the task assigned to the worker
	Files    []string // address of the files the worker has to work on
	NReduce  int      // the number of reduce tasks that the worker will need to divide its output in
}

type TaskDoneArgs struct {
	Id       int            // id of worker
	TaskType int            // The type of the task
	TaskId   int            // task id that has been completed
	Files    map[int]string // maps the files to the corresponding reduce task
}

type TaskDoneReply struct {
	Id int // Okay
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
