package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"encoding/gob"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"../labrpc"
)

// import "bytes"
// import "../labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

const LEADER = 1
const FOLLOWER = 2
const CANDIDATE = 3

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu          sync.Mutex          // Lock to protect shared access to this peer's state
	peers       []*labrpc.ClientEnd // RPC end points of all peers
	persister   *Persister          // Object to hold this peer's persisted state
	me          int                 // this peer's index into peers[]
	dead        int32               // set by Kill()
	currentTerm int
	state       int // determines what kind of server this current one is
	votedFor    int
	log         []LogEntry
	grantedVote chan bool
	heartbeat   chan bool
	leaderchan  chan bool
	commitchan  chan bool
	//Volatile state variables
	commitIndex int
	lastApplied int
	//Volatile state only on leaders (reinitialized after election)
	nextIndex  []int
	matchIndex []int
}

//
// A model for a long entry in state of the machines
//
type LogEntry struct {
	command string
	termID  int
	logID   int
	logTerm int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	var term int
	isleader := false
	term = rf.currentTerm
	if rf.state == LEADER {
		isleader = true
	}
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	//Example:
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	d.Decode(&rf.currentTerm)
	d.Decode(&rf.votedFor)
	d.Decode(&rf.log)
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	Term         int //which term it is requesting the vote for
	CandidateID  int //candidate requesting the vote
	LastLogIndex int //index of the last log entry
	LastLogTerm  int //term of candidate's last log entry
}

//
//Reply false if term<currentTerm
// field names must start with capital letters!
//
type RequestVoteReply struct {
	Term        int
	VoteGranted bool //true means the candidate received a vote
}

//
// example RequestVote RPC handler.
//
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

func (rf *Raft) lastIndex() int {
	return rf.log[len(rf.log)-1].logID
}
func (rf *Raft) lastTerm() int {
	return rf.log[len(rf.log)-1].logID
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := rf.currentTerm
	isleader := false
	if rf.state == LEADER {
		isleader = true
	}
	if isleader {
		index = rf.lastIndex() + 1
		rf.log = append(rf.log, LogEntry{termID: term}) // append new entry from client
		rf.persist()
	}

	return index, term, isleader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.state = FOLLOWER
	rf.heartbeat = make(chan bool, 100)
	rf.votedFor = -1
	rf.log = append(rf.log, LogEntry{logTerm: 0})
	rf.currentTerm = 0
	rf.commitchan = make(chan bool, 100)
	rf.grantedVote = make(chan bool, 100)
	rf.leaderchan = make(chan bool, 100)
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	go func() { //start t
		for {
			if rf.state == FOLLOWER {
				select {
				case <-rf.heartbeat:
				case <-rf.grantedVote:
				case <-time.After(time.Duration(rand.Int63()%333+550) * time.Millisecond):
					rf.state = CANDIDATE //after a certain amount of time, it becomes a candidate
				}
			}
			if rf.state == LEADER {
				//fmt.Printf("Leader:%v %v\n",rf.me,"boatcastAppendEntries	")
				time.Sleep(1000)
			}
			if rf.state == CANDIDATE {
				rf.mu.Lock()
				rf.currentTerm++
				rf.votedFor = rf.me
				rf.persist()
				rf.mu.Unlock()
				//fmt.Printf("%v become CANDIDATE %v\n",rf.me,rf.currentTerm)
				select {
				case <-time.After(time.Duration(rand.Int63()%333+550) * time.Millisecond):
				case <-rf.heartbeat:
					rf.state = FOLLOWER
				//	fmt.Printf("CANDIDATE %v reveive chanHeartbeat\n",rf.me)
				case <-rf.leaderchan:
					rf.mu.Lock()
					rf.state = LEADER
					//fmt.Printf("%v is Leader\n",rf.me)
					rf.nextIndex = make([]int, len(rf.peers))
					rf.matchIndex = make([]int, len(rf.peers))
					for i := range rf.peers {
						rf.nextIndex[i] = rf.lastIndex() + 1
						rf.matchIndex[i] = 0
					}
					rf.mu.Unlock()
					//rf.boatcastAppendEntries()
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-rf.commitchan:
				rf.mu.Lock()
				commitIndex := rf.commitIndex
				baseIndex := rf.log[0].logID
				for i := rf.lastApplied + 1; i <= commitIndex; i++ {
					msg := ApplyMsg{Command: rf.log[i-baseIndex].command}
					applyCh <- msg
					//fmt.Printf("me:%d %v\n",rf.me,msg)
					rf.lastApplied = i
				}
				rf.mu.Unlock()
			}
		}
	}()
	return rf
}
