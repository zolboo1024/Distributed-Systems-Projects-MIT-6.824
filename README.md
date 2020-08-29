Various Distributed Systems Projects (Guided by MIT-6.824)
================================================================================
All of these programs are written in the language **Go** so if you want to run any of these, please make sure you have that installed. Also, some of the 
plugin modules that are used may not be compatible with the most updated version of **Go** when you read this, so please use **Go** v1.12 or v1.13 to be compatible 
with the plugins. The code snippets are truncated to provide simplicity and only highlight the important parts.


## Project: MapReduce WordCounter
MapReduce is a program that utilizes a distributed system of two different threads (would be many many machines in the case of large-scale, real-world application) 
to count the number of occurences of unique words in a .txt file. The worker threads Maps the input (puts each word in a map with the value being the occurence) and sends them to the master. The worker threads also Reduces (hence the name) meaning that it tallies up everything into a single map instance to count the total. To learn more about MapReduce and how it was implemented at Google, check out this paper by Jeffrey Dean and Sanjay Ghemawat: [paper](https://pdos.csail.mit.edu/6.824/papers/mapreduce.pdf).
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

## Project: Go Raft

Raft is a distributed consensus algorithm that is developed by Diego Ongaro and John Ousterhout at Stanford University (more details provided in their own [paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf)). It runs on replicated servers and implements a leader-based approach to provide efficient fault-tolerance. The main point of this algorithm is that a leader server acts as a primary server and provides only a log of clients' commands to the replicas so that they stay up to date. In order to be a leader, a server has to be elected (constrained by electionTimer and majority vote) and the leader responsibility essentially rotates. Unlike Byzantine consensus algorithms, the leader's logs cannot be contested- it holds absolute power over the replicas and therefore provides efficiency. My entire Raft implementation is in the **src/raft/raft.go** file. In order to run check this program out, you can run the test file (provided by the MIT professors) which produces test outputs:
```bash
$ cd src/raft
$ go test
```
### A brief overview of my implementation
Let's look at the code through a simple event of when a client requests the server to execute a given command:
The servers (the primaries and the replicas) are all first initialized. In order to initialize the servers, **Make()** method is called:
```bash
func Make(...[omitted for simplicity's sake]) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.state = FOLLOWER
	rf.heartbeat = make(chan bool, 100)
	rf.votedFor = -1
	...
	rf.currentTerm = 0
	...
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	...
```
We initialize the servers by initializing a Raft server's state variables that are defined earlier. After this, the make() method also immediately sends commands to check if a vote has been granted or not (more details in the [paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf) and in the source code). 
Once the servers are initialized, the leader sends an appendEntries() request to the followers so that they stay up to date on the logs:
```bash
func (rf *Raft) AppendEntries(args AppendEntriesArgs, reply *AppendEntriesReply) {
	if args.Term < rf.currentTerm { //since the term does not match and lags behind
		reply.Successful = false
		return
	}
	...
	if args.LeaderCommit == rf.commitIndex { //if the terms check out, we are ready to append the entries
		rf.log = rf.log[:args.PrevLogIndex+1-base]
		rf.log = append(rf.log, args.Entries...) //append the entries to the replicas
		reply.Successful = true
		reply.NextIndex = rf.lastIndex() + 1
	}
	...
}
```
Then, once the majority of the followers respond that they have received the appendEntries() request, the leader executes the client's command and replies. Now, there terms and elections. Each term is timed and when it expires, a new election must begin. A candidate requests votes from its peers:
```bash
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.VoteGranted = false
	if args.Term != rf.currentTerm { //check if the requester term is up to date
		reply.Term = rf.currentTerm
		return
	}
	term := rf.lastTerm()
	index := rf.lastIndex()
	if args.LastLogTerm > term || (args.LastLogTerm == term && args.LastLogIndex >= index) {
		// if it is up to date and it has not voted, request the vote
		if rf.votedFor == -1 || rf.votedFor == args.CandidateID {
			rf.grantedVote <- true
			rf.state = FOLLOWER
			reply.VoteGranted = true
			rf.votedFor = args.CandidateID
		}
	}
}
```
The election proceeds in this manner and if there is any tie between leader candidates, Raft says that the tie is split randomly so there is no delay. 
