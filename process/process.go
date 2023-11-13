package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	proto "grpc/GRPC"

	"google.golang.org/grpc"
)

type Process struct {
	proto.QueueServiceServer
	PeerProcesses     map[int64]proto.QueueServiceClient
	context           context.Context
	id                int64
	timestamp         int64
	request           *proto.AccessRequest
	inCriticalSection bool
	inQueue           bool
}

func main() {
	//Set process port from command line argument (use as ID):
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 64)
	port := int64(arg1) + 5000

	context, cancel := context.WithCancel(context.Background())
	defer cancel()

	//create process
	process := &Process{
		id:                port,
		timestamp:         int64(1),
		context:           context,
		PeerProcesses:     make(map[int64]proto.QueueServiceClient),
		inCriticalSection: false,
		inQueue:           false,
	}

	f, err := os.OpenFile("../log.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666) //create log file
	if err != nil {
		log.Fatalf("error opening file: %v \n", err)
		fmt.Printf("error opening file: %v \n", err)
	}
	defer f.Close()
	log.SetOutput(f) //set log output to file

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port)) //create listener
	if err != nil {
		log.Fatalf("error creating the process %v \n", err)
		fmt.Printf("error creating the process %v \n", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterQueueServiceServer(grpcServer, process)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			fmt.Printf("error serving the process %v \n", err)
		}
	}()

	for i := 0; i < 3; i++ {
		peer_port := int64(i) + int64(5000)

		if peer_port == port {
			continue
		}

		var connection *grpc.ClientConn
		fmt.Printf("Trying to connect to process port: %d\n", peer_port)
		connection, err := grpc.Dial(fmt.Sprintf(":%d", peer_port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("error connecting to process %v \n", err)
			fmt.Printf("error connecting to process %v \n", err)
		}
		defer connection.Close()

		process.PeerProcesses[peer_port] = proto.NewQueueServiceClient(connection) // add peer process to map
	}

	for {
		random := rand.Intn(10)

		if random < 3 {
			process.RequestCriticalSection() //request critical section

			process.EnterCriticalSection() //enter critical section

			process.CriticalSection() //does something cool in critical section

			process.ExitCriticalSection() //exit critical section
		}
	}
}

func (process *Process) RequestCriticalSection() {
	request := &proto.AccessRequest{
		Id:        process.id,
		Timestamp: process.timestamp,
	}
	process.request = request
	process.inQueue = true
	log.Printf("(id: %d) sending request to all peers at lamport time %d \n", process.id, process.timestamp)
	fmt.Printf("(id: %d) sending request to all peers at lamport time %d \n", process.id, process.timestamp)

	for id, peer := range process.PeerProcesses { //send request to all peers
		_, err := peer.RequestAccess(process.context, request)
		if err != nil {
			log.Fatalf("error sending request to (id: %d): %v \n", id, err)
			fmt.Printf("error sending request to (id: %d): %v \n", id, err)
		}
		log.Printf("(id: %d) received response from (id: %d) at lamport time %d \n", process.id, id, process.timestamp)
		fmt.Printf("(id: %d) received response from (id: %d) at lamport time %d \n", process.id, id, process.timestamp)
	}
	process.timestamp++

}

func (process *Process) RequestAccess(context context.Context, request *proto.AccessRequest) (*proto.AccessResponse, error) {
	process.timestamp++
	fmt.Printf("(id: %d) received request from (id: %d) at lamport time %d \n", process.id, request.Id, request.Timestamp)
	log.Printf("(id: %d) received request from (id: %d) at lamport time %d \n", process.id, request.Id, request.Timestamp)

	// wait for critical section to be available
	for !process.CriticalSectionAvailable(request) {
	}

	//send ack to process who requested
	fmt.Printf("(id: %d) sending ack to (id: %d) at lamport time %d\n", process.id, request.Id, process.timestamp)
	log.Printf("(id: %d) sending ack to (id: %d) at lamport time %d\n", process.id, request.Id, process.timestamp)
	response := &proto.AccessResponse{}
	return response, nil
}

func (process *Process) CriticalSectionAvailable(receivedRequest *proto.AccessRequest) bool {
	if process.inCriticalSection {
		return false
	}

	if process.inQueue {
		if process.request.Timestamp < receivedRequest.Timestamp || (process.request.Timestamp == receivedRequest.Timestamp && process.request.Id < receivedRequest.Id) { // if process is in queue and recieved request has lower priority
			return false
		}
	}

	return true
}

func (process *Process) EnterCriticalSection() {
	process.inCriticalSection = true
	fmt.Printf("Process %d entered critical section\n", process.id)
	log.Printf("Process %d entered critical section\n", process.id)
}

func (process *Process) CriticalSection() {
	time.Sleep(3 * time.Second) //simulate critical section
}

func (process *Process) ExitCriticalSection() {
	fmt.Printf("Process %d exited critical section\n", process.id)
	log.Printf("Process %d exited critical section\n", process.id)
	process.inQueue = false
	process.inCriticalSection = false
}
