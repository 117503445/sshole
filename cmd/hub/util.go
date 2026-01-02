package main

import (
	"net"
)

// findFreePort 找到一个本地可用的 tcp port
func findFreePort() (int32, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return int32(listener.Addr().(*net.TCPAddr).Port), nil
}
