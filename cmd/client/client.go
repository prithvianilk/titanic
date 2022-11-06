package main

import (
	"fmt"
	"net/rpc"
	"os"

	"github.com/prithvianilk/titanic/internal"
)

const (
	getCommand = "get"
	putCommand = "put"
)

func main() {
	commandType := os.Args[1]
	leaderAddr := internal.LocalHostAddr + ":" + os.Args[2]
	client, err := rpc.DialHTTP("tcp", leaderAddr)
	if err != nil {
		fmt.Printf("Failed to establish connection with leader (%s)\n", leaderAddr)
		return
	}
	if commandType == getCommand {
		key := os.Args[3]
		var value string
		err := client.Call("Titanic.Get", key, &value)
		if err != nil {
			fmt.Printf("Get request failed: %v\n", err)
		}
		fmt.Println(value)
	} else if commandType == putCommand {
		key, value := os.Args[3], os.Args[4]
		kvPair := internal.KVPair{Key: key, Value: value}
		err := client.Call("Titanic.Put", kvPair, nil)
		if err != nil {
			fmt.Printf("Put request failed: %v\n", err)
		}
	} else {
		fmt.Println("Incorrect command type: " + commandType)
	}
}
