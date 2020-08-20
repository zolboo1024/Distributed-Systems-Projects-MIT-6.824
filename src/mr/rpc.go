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

// RPCArgs Argument struct for RPC Calls
// The RPCType indicates what kind of call
// this specified call is.
// if WORKER_NEEDTASK, it is a call asking for map or reduce
// if WORKER_MAPRESULT, it is a call sharing the result of mapping
// if WORKER_REDUCERESULT, it is a call sharing the result of reducing
//
type RPCArgs struct {
	RPCType    int
	Mapped     []KeyValue
	Reduced    KeyValue
	ReducedArr []KeyValue
}

// RPCReply The WorkType indicates what kind of call
// this specified call is.
// if MASTER_ASSIGNMAP, it is a call assigning the job of mapping
// in this case, it provides an array of strings to be mapped
// if MASTER_ASSIGNREDUCE, it is a call assigning the job of reducing
// in this case, it provides an array of maps to be reduced
// if MASTER_EMPTYTASK, it means there is nothing to assign to the worker at the time
type RPCReply struct {
	WorkType     int
	MapInput     string
	ReduceArr    []KeyValue
	ReduceStart  int
	ReduceFinish int
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
