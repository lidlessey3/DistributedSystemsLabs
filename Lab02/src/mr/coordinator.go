package mr

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

type taskData struct {
	taskLock   *sync.Mutex // Mutex for modifying the data
	taskId     int         // the id of the task
	taskType   int         // type of task 1 if map, 2 if reduce
	files      []string    // file it is bound to
	done       bool        // if it has been executed
	lastIssued time.Time   // time stamp of the last time it was issued to a worker
	issued     bool        // whether is has already been issued
}

type Coordinator struct {
	workLock    sync.Mutex // Mutex to guard task completion tasks
	nReduce     int        // Total number of reduce tasks
	nMap        int        // Number of map tasks
	workDone    int        // Number of tasks that have been completed
	tasks       []taskData // List of all the tasks that need to be done
	workers     int        // counter of workers
	workersLock sync.Mutex // worker mutex
}

func (c *Coordinator) AskTask(args *AskTaskArgs, reply *AskTaskReply) error {
	if args.Id == -1 {
		c.workersLock.Lock()
		reply.Id = c.workers
		c.workers++
		c.workersLock.Unlock()
	} else { // it is an already existing worker
		reply.Id = args.Id
	}

	reply.TaskId = -1

	// Based on whether we are at the moment only working on map or reduce tasks We change
	// The beginning and end points of the for loop
	start := 0
	end := c.nMap + c.nReduce

	c.workLock.Lock()
	if c.workDone < c.nMap {
		end = c.nMap
	} else {
		start = c.nMap
	}
	c.workLock.Unlock()

	// Iterate over the tasks using the indexes start and end
	for i := start; i < end; i++ {
		c.tasks[i].taskLock.Lock()
		// found an unassigned task or a task that was last assigned more than 10 seconds ago
		if !c.tasks[i].done && (!c.tasks[i].issued || time.Since(c.tasks[i].lastIssued) > 10*time.Second) {
			c.tasks[i].lastIssued = time.Now()
			c.tasks[i].issued = true
			reply.NReduce = c.nReduce
			if start == 0 { // If we are doing a map we pass the original id
				reply.TaskId = c.tasks[i].taskId
			} else { // if it is a reduce tasks we need to remove the remove the number of tasks to get the absolute id of it
				reply.TaskId = c.tasks[i].taskId - c.nMap // this is necessary because the worker expects the id of the reduce to start at 0
			}
			reply.TaskType = c.tasks[i].taskType
			reply.Files = append(reply.Files, c.tasks[i].files...)
			c.tasks[i].taskLock.Unlock()
			break
		}
		c.tasks[i].taskLock.Unlock()
	}

	return nil
}

func (c *Coordinator) TaskDone(args *TaskDoneArgs, reply *TaskDoneReply) error {
	reply.Id = args.Id
	actualTaskId := args.TaskId
	if args.TaskType == 2 {
		actualTaskId += c.nMap
	}

	if actualTaskId > c.nMap+c.nReduce {
		return errors.New("task not found")
	}

	c.tasks[actualTaskId].taskLock.Lock()
	if !c.tasks[actualTaskId].done { // check that this tasks has not been completed by another worker
		c.tasks[actualTaskId].done = true // Set it to done
		c.workLock.Lock()                 // need this to modify the workDone variable and modify all the tasks array before setting it
		c.workDone++

		// When map task finished
		// need to add the generated intermediate files to the reduce tasks
		if args.TaskType == 1 {
			for j := c.nMap; j < c.nMap+c.nReduce; j++ {
				file, exists := args.Files[j-c.nMap]
				if exists {
					c.tasks[j].files = append(c.tasks[j].files, file)
				}
			}
		}

		c.workLock.Unlock()
	}
	c.tasks[actualTaskId].taskLock.Unlock()
	return nil
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	var port int
	var l net.Listener
	var e error
	portStr, available := os.LookupEnv("COORDINATOR_PORT")
	if available {
		port, _ = strconv.Atoi(portStr)
		l, e = net.Listen("tcp", fmt.Sprintf(":%d", port))

	} else {
		sockname := coordinatorSock()
		os.Remove(sockname)
		l, e = net.Listen("unix", sockname)
	}
	if e != nil {
		log.Fatal("listen error:", e)
	}

	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	ret := true

	for i := 0; i < len(c.tasks); i++ { // check if all tasks have been completed
		c.tasks[i].taskLock.Lock()
		if !c.tasks[i].done {
			c.tasks[i].taskLock.Unlock()
			ret = false
			break
		}
		c.tasks[i].taskLock.Unlock()
	}

	return ret
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	c := Coordinator{}

	// create task temporary for now one task per file
	c.nReduce = nReduce
	c.nMap = len(files)
	c.workDone = 0
	for i := 0; i < len(files); i++ {
		newTask := taskData{}
		newTask.taskLock = new(sync.Mutex)
		newTask.taskId = i
		newTask.taskType = 1
		newTask.files = append(newTask.files, files[i])
		newTask.done = false
		newTask.issued = false
		c.tasks = append(c.tasks, newTask)
	}

	for i := 0; i < nReduce; i++ {
		newTask := taskData{}
		newTask.taskLock = new(sync.Mutex)
		newTask.taskId = i + c.nMap
		newTask.taskType = 2
		newTask.done = false
		newTask.issued = false
		c.tasks = append(c.tasks, newTask)
	}

	c.server()
	return &c
}
