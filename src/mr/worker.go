package mr

import (
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
)

//
// KeyValue : Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	//the very first worker RPC call to the master
	initialRPC := RPCArgs{}
	initialRPC.RPCType = 0
	currentreply := CallMaster(&initialRPC)
	worktype := currentreply.WorkType
	//as long as the master has task to give, it keeps working
	for worktype == MASTER_ASSIGNMAP || worktype == MASTER_ASSIGNREDUCE {
		if worktype == MASTER_ASSIGNMAP {
			//in this case, the master assigns mapping task
			//so, the worker opens up the file and maps it
			filename := currentreply.MapInput
			file, err := os.Open(currentreply.MapInput)
			if err != nil {
				log.Fatalf("cannot open %v", filename)
			}
			content, err := ioutil.ReadAll(file)
			if err != nil {
				log.Fatalf("cannot read %v", filename)
			}
			file.Close()
			kva := mapf(filename, string(content))
			//return the mapped instance of this file
			rpctoreturn := RPCArgs{}
			rpctoreturn.RPCType = WORKER_MAPRESULT
			rpctoreturn.Mapped = kva
			_ = CallMaster(&rpctoreturn)
		} else if worktype == MASTER_ASSIGNREDUCE {
			//in this case, the master assigns reducing task
			//so, the worker reduces the intermediate arr using the specified key
			values := []string{}
			reducedarr := currentreply.ReduceArr[:]
			i := currentreply.ReduceStart
			j := currentreply.ReduceFinish
			for k := i; k < j; k++ {
				values = append(values, reducedarr[k].Value)
			}
			output := reducef(reducedarr[i].Key, values)
			// this is the format for each line of Reduce output.
			oname := "mr-out-0"
			ofile, _ := os.Create(oname)
			fmt.Fprintf(ofile, "%v %v\n", reducedarr[i].Key, output)
		}
		currentreply = CallMaster(&initialRPC)
		worktype = currentreply.WorkType
	}
}

// CallMaster Function makes RPC call to the master
// Calls the RPCHandler function in the master asking for a task
// and then waits for a reply
func CallMaster(args *RPCArgs) *RPCReply {
	//Declare a type of RPCReply to save the reply
	reply := RPCReply{}
	//send the RPC request and wait for the reply
	call("Master.RPCHandler", args, &reply)
	return &reply
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
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
