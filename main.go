package main

import (
	"fmt"
	"hello/util"
	"log"
)

/*
*
gcc -fPIC -shared -o libucli_ffi.dylib ucli_ffi.c
*/
func main() {
	// Initialize market publisher
	marketPublisher, err := util.NewMarketPublisher("market_topic")
	if err != nil {
		log.Fatalf("Failed to create market publisher: %v", err)
	}
	defer marketPublisher.Close()

	// Publish a market message
	err = marketPublisher.MarketPublish("Hello, Market!")
	if err != nil {
		log.Fatalf("Failed to publish market message: %v", err)
	}

	// Initialize market receiver
	marketReceiver, err := util.NewMarketReceiver("market_topic")
	if err != nil {
		log.Fatalf("Failed to create market receiver: %v", err)
	}
	defer marketReceiver.Close()

	// Receive a market message
	message := marketReceiver.MarketReceive(1024)
	if err != nil {
		log.Fatalf("Failed to receive market message: %v", err)
	}
	fmt.Println("Received market message:", message)
}
