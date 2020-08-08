Various Distributed Systems Projects (Guided by MIT-6.824)
================================================================================
All of these programs are written in the language **Go** so if you want to run any of these, please make sure you have that installed. Also, some of the 
plugin modules that are used may not be compatible with the most updated version of **Go** when you read this, so please use **Go** v1.12 or v1.13 to be compatible 
with the plugins.


## Project: MapReduce WordCounter
MapReduce is a program that utilizes a distributed system of two different threads (would be many many machines in the case of large-scale, real-world application) 
to count the number of occurences of unique words in a .txt file. The worker threads Maps the input (puts each word in a map with the value being the occurence) and 
sends them to the master. The worker threads also Reduces (hence the name) meaning that it tallies up everything into a single map instance to count the total.
To test this program out, in the **/src/main** directory, run:
```bash
$ go build -buildmode=plugin ../mrapps/wc.go
$ go run mrmaster.go pg-*.txt
```
to get a freshly built plugin and run the master thread. Then, in a new terminal window:
```bash
$ go run mrworker.go wc.so
```
to start the worker. Once the job is finished, you will see mr-out-0.txt file with the output. The output contains each unique word in the test files (a bunch of classic
books) and the number of occurence for each word next to it. 
### A brief overview of my implementation
For this program, I had to basically had to implement 2 separate programs: a program for the master and a program for the worker. The master is the head of the 
operation that gives keeps track of the process and gives other worker threads tasks to complete. The master thread and the worker thread communicate through an 
RPC call. So, in the master program (/src/mr/master.go) a local server is set up by the master to handle the communication:
```bash
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
```
which handles RPC calls from the workers.So, if a worker thread is available for work, it sends RPC to the master, asking for work:
```bash
func CallMaster(args *RPCArgs) *RPCReply {
	//Declare a type of RPCReply to save the reply
	reply := RPCReply{}
	//send the RPC request and wait for the reply
	call("Master.RPCHandler", args, &reply)
	return &reply
}
```
On the master's side, we also need a function to listen to the workers and respond. In this implementation, I designed types of calls such as 
workers requesting tasks, or returning what they processed. For example, if the master confirms that the mapping is done and is ready to 
assign reducing tasks, the implementation is:
```bash
reply.WorkType = 1
j := m.CurMapped + 1 //get the current checkpoint
for j < len(m.Mapped) && m.Mapped[j].Key == m.Mapped[m.CurMapped].Key {
	j++
}
reply.ReduceArr = m.Mapped
reply.ReduceStart = m.CurMapped
reply.ReduceFinish = j
m.CurMapped = j
```
Here, the Master keeps track of which part of the map is reduced and continuously assigns the unreduced parts to workers to process. The rest of the program
is similar to this. 
