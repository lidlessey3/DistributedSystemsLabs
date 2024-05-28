package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
// Handle failures on the communication with the coordinator
// By assuming that the job was done so the worker terminates itself
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	id := -1
	for {
		// Ask for a task from the coordinator
		work, err := AskTask(id)
		if err != nil {
			log.Fatal()
			os.Exit(1)
		}
		id = work.Id

		// Stay idle, job is not done
		if work.TaskId == -1 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		outputFiles := make(map[int]string, 0)

		switch work.TaskType {
		// map task
		case 1:
			fileContent, err := os.ReadFile(work.Files[0])
			if err != nil {
				log.Fatal()
				os.Exit(1)
			}
			mapResult := mapf(work.Files[0], string(fileContent))

			// now distribute the result to the reduce files
			// A reasonable naming convention for intermediate files is mr-X-Y, where X is the Map task number, and Y is the reduce task number.

			outputLists := make(map[int][]KeyValue, 0)

			for i := 0; i < len(mapResult); i++ {
				indexOfFile := ihash(mapResult[i].Key) % work.NReduce
				_, exists := outputLists[indexOfFile]
				if !exists {
					outputLists[indexOfFile] = make([]KeyValue, 0)
				}
				outputLists[indexOfFile] = append(outputLists[indexOfFile], mapResult[i])
			}

			for i := 0; i < work.NReduce; i++ {
				data, exists := outputLists[i]
				if exists {
					outputFiles[i] = fmt.Sprintf("mr-tmp-%d-%d-%d", id, work.TaskId, i)
					file, err := os.Create(outputFiles[i])
					if err != nil {
						log.Fatal()
						os.Exit(1)
					}
					err = json.NewEncoder(file).Encode(data)
					if err != nil {
						log.Fatal()
						os.Exit(1)
					}
					file.Close()
				}
			}

		// Reduce task
		case 2:
			// read the files from the received paths and put them in memory
			// then sort the data by key and for each key, put all the associated values in one array
			// call reducef for each key and its associated array and put the results in one file mr-out-taskId

			if len(work.Files) == 0 { // if there are no input files break
				break
			}

			contentData := make([]KeyValue, 0)
			for i := 0; i < len(work.Files); i++ {
				readFile, err := os.Open(work.Files[i])
				if err != nil {
					log.Fatal()
					os.Exit(1)
				}
				fileScanner := json.NewDecoder(readFile)

				kVL := make([]KeyValue, 0)
				err = fileScanner.Decode(&kVL)
				if err != nil {
					log.Fatal()
					os.Exit(1)
				}

				contentData = append(contentData, kVL...)

				readFile.Close()
			}

			// sort contentData by key
			sort.Slice(contentData, func(i, j int) bool {
				return contentData[i].Key < contentData[j].Key
			})
			tmpName := fmt.Sprintf("mr-tmp-%d-%d", id, work.TaskId)
			file, err := os.Create(tmpName)
			if err != nil {
				log.Fatal()
				os.Exit(1)
			}

			if len(contentData) > 0 {
				currentKey := contentData[0].Key
				var values []string
				values = append(values, contentData[0].Value)
				for i := 1; i < len(contentData); i++ {
					if contentData[i].Key == currentKey {
						values = append(values, contentData[i].Value)
					} else {
						reduceResult := reducef(currentKey, values)
						file.WriteString((currentKey + " " + reduceResult + "\n"))

						values = values[:0]
						currentKey = contentData[i].Key
						values = append(values, contentData[i].Value)
					}
				}
				reduceResult := reducef(currentKey, values)
				file.WriteString((currentKey + " " + reduceResult + "\n"))
			}
			file.Close()
			outputFiles[0] = fmt.Sprintf("mr-out-%d.txt", work.TaskId)
			os.Rename(tmpName, outputFiles[0]) // On linux this should be atomic
		}

		err = WorkDone(id, work.TaskId, work.TaskType, outputFiles)

		if err != nil {
			os.Exit(0)
		}
	}
}

// Send RPC call to coordinator asking for a task
// Reply includes files to use as input, id of task and type of task
func AskTask(id int) (*AskTaskReply, error) {

	// declare an argument structure.
	args := AskTaskArgs{}

	// fill in the argument(s).
	args.Id = id

	// declare a reply structure.
	reply := AskTaskReply{}

	ok := call("Coordinator.AskTask", &args, &reply)
	if ok {
		return &reply, nil
	} else {
		return nil, errors.New("there was an error talking to the server")
	}
}

// Send RPC call to coordinator indicating that task with taskId is done
// The call includes the paths to the files created for the task
func WorkDone(id int, taskId int, taskType int, outputFiles map[int]string) error {
	// declare an argument structure.
	args := TaskDoneArgs{}

	// fill in the argument(s).
	args.Id = id
	args.TaskId = taskId
	args.TaskType = taskType
	args.Files = outputFiles

	// declare a reply structure.
	reply := TaskDoneReply{}

	ok := call("Coordinator.TaskDone", &args, &reply)
	if ok {
		return nil
	} else {
		return errors.New("there was an error talking to the server")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	var c *rpc.Client
	var err error
	coordinatorIp, available := os.LookupEnv("COORDINATOR_IP")
	if available {
		c, err = rpc.DialHTTP("tcp", coordinatorIp)
	} else {
		sockname := coordinatorSock()
		c, err = rpc.DialHTTP("unix", sockname)
	}
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
