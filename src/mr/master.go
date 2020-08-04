package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
)

//
// Master strut: Files contain all the processed words
// NReduce defines how many reduces to commit
// Mapped contains all the intermediate pairs
// Reduced contains all the reduced pairs
//
type Master struct {
	// SplitFiles contain the values of the strings to be processed
	// split into m buckets of size n (the string array is m*n)
	// nReduce and files in the MakeMaster
	// method specifies these variables.
	Files      []string
	CurFiles   int
	NReduce    int
	Mapped     []KeyValue
	CurMapped  int
	Reduced    []KeyValue
	CurReduced int
}

// RPCHandler is called by the worker to request
// Various RPC calls
func (m *Master) RPCHandler(args *RPCArgs, reply *RPCReply) error {
	//In this case, we give tasks
	if args.RPCType == 0 {
		//In this case, send a word to be mapped
		if m.CurFiles < len(m.Files) {
			reply.WorkType = 0
			reply.MapInput = m.Files[m.CurFiles]
			m.CurFiles = m.CurFiles + 1
		}
		//In this case, send all the specific mapped inputs to be reduced
		if m.CurMapped < m.NReduce {
			reply.WorkType = 1
			//come back to this
		}

	} else {
		//In this case, the RPC was called by the worker to send processed output
		//we handle their input and store it into a mapped or reduced array
		//In this case, receive the mapped key pair and append it to the Mapped array
		if args.RPCType == 1 {
			m.Mapped = append(m.Mapped, args.Mapped)
		}
		//In this case, receive the reduced key pair and append it to the Reduced array
		if args.RPCType == 2 {
			m.Reduced = append(m.Reduced, args.Reduced)
		}
	}
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	ret := false

	// Your code here.

	return ret
}

//
// MakeMaster create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}
	//sizem is how large the 2d array is (how many splits to make)
	//sicne nReduce essentially tells you how long each split should be
	m.NReduce = nReduce
	m.Files = files
	m.CurFiles = 0
	m.CurMapped = 0
	m.CurReduced = 0
	mapped := []string{}
	reduced := []string{}
	m.Mapped = mapped
	m.Reduced = reduced
	m.server()
	return &m
}
