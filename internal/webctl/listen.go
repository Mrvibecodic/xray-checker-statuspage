package webctl

import "net"

func netListen(addr string) (net.Listener, error) { return net.Listen("tcp", addr) }
