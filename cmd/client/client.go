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
		panic(err)
	}
	if commandType == getCommand {
		key := os.Args[3]
		var value string
		err := client.Call("Get", key, &value)
		if err != nil {
			panic(err)
		}
		fmt.Println(value)
	} else if commandType == putCommand {
		key, value := os.Args[3], os.Args[4]
		kvPair := internal.KVPair{key, value}
		var success bool
		err := client.Call("Put", kvPair, &success)
		if err != nil {
			panic(err)
		} else if !success {
			fmt.Println("PUT request failed")
		}
	} else {
		fmt.Println("Incorrect command type: " + commandType)
	}
}
