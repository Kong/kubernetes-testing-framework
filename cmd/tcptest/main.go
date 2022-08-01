package main

// This is the code to build image for running jobs to test TCP connectivity of service.

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	DefaultTestTimeout    = 60 * time.Second
	DefaultConnectTimeout = 3 * time.Second
)

func main() {

	var address string
	testTimeout := DefaultTestTimeout
	connectTimeout := DefaultConnectTimeout

	flag.StringVar(&address, "test-address", "127.0.0.1:80", "test connectivity of this address")
	flag.DurationVar(&testTimeout, "test-timeout", DefaultTestTimeout, "timeout before successfully connected to test address")
	flag.DurationVar(&connectTimeout, "connect-timeout", DefaultConnectTimeout, "timeout for a single connect")
	flag.Parse()

	timeoutTimer := time.NewTimer(testTimeout)
	for {
		select {
		case <-timeoutTimer.C:
			fmt.Printf("ERROR: timed out to connect to %s\n", address)
			os.Exit(1)
		default:
			_, err := net.DialTimeout("tcp", address, connectTimeout)
			if err == nil {
				fmt.Printf("INFO: succeeded to connect to %s\n", address)
				return
			}
			// TODO: directly exit with non-0 code for some errors that can fail without retry, e.g: bad address
		}
	}
}
