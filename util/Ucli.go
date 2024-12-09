// ucli/ucli.go
package util

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lucli_ffi
#include "ucli_ffi.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"unsafe"
)

type MarketPublisher struct {
	publisher C.struct_UltraMarketPublisher
	topic     *C.char
}
type MarketReceiver struct {
	receiver C.struct_UltraMarketReceiver
	topic    *C.char
}
type OrderPublisher struct {
	publisher C.struct_UltraOrderPublisher
	topic     *C.char
}
type OrderReceiver struct {
	receiver C.struct_UltraOrderReceiver
	topic    *C.char
}

// NewMarketPublisher 创建并初始化一个新的MarketPublisher实例。
// 参数:
//
//	topic: 用于标识发布者主题的字符串。不能为空。
//
// 返回值:
//
//	*MarketPublisher: 一个指向新创建的MarketPublisher实例的指针。
//	error: 如果内存分配失败或初始化过程中出现错误，返回一个错误。
func NewMarketPublisher(topic string) (*MarketPublisher, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("NewMarketPublisher:trans go.msg to c.msg error")
	}
	var publisher C.struct_UltraMarketPublisher
	C.init_market_publisher(cTopic, &publisher)
	return &MarketPublisher{
		publisher: publisher,
		topic:     cTopic,
	}, nil
}

func NewOrderPublisher(topic string) (*OrderPublisher, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("NewOrderPublisher:trans go.msg to c.msg error")
	}
	var publisher C.struct_UltraOrderPublisher
	C.init_order_publisher(cTopic, &publisher)
	return &OrderPublisher{
		publisher: publisher,
		topic:     cTopic,
	}, nil
}

// MarketPublish 发布消息到市场。
// 参数 msg 是要发布的消息内容。
// 返回值 error 用于指示消息发布过程中是否遇到了错误。
// 该函数首先将 Go 字符串转换为 C 字符串，以供 C 语言库使用。
// 如果转换失败（例如，由于内存分配问题），则返回错误。
// 在发布消息之前，检查 C 字符串是否成功分配了内存，
// 确保在使用后释放内存以避免内存泄漏。
func (mp *MarketPublisher) MarketPublish(msg string) error {
	cMsg := C.CString(msg)
	if cMsg == nil {
		return errors.New("MarketPublish:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cMsg))
	C.publish_market(&mp.publisher, cMsg)
	return nil
}
func (op *OrderPublisher) OrderPublish(msg string) error {
	cMsg := C.CString(msg)
	if cMsg == nil {
		return errors.New("OrderPublish:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cMsg))
	C.publish_order(&op.publisher, cMsg)
	return nil
}

func (mp *MarketPublisher) Close() {
	C.free(unsafe.Pointer(mp.topic))
}
func (op *OrderPublisher) Close() {
	C.free(unsafe.Pointer(op.topic))
}

// NewMarketReceiver 创建并初始化一个新的MarketReceiver实例。
// 参数:
//
//	topic: 用于接收市场数据的主题字符串。
//
// 返回值:
//
//	*MarketReceiver: 初始化后的MarketReceiver实例指针。
//	error: 如果内存分配失败或初始化过程中发生错误，返回相应的错误。
func NewMarketReceiver(topic string) (*MarketReceiver, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("NewMarketReceiver:trans go.msg to c.msg error")
	}
	var receiver C.struct_UltraMarketReceiver
	C.init_market_receiver(cTopic, &receiver)
	return &MarketReceiver{
		receiver: receiver,
		topic:    cTopic,
	}, nil
}
func NewOrderReceiver(topic string) (*OrderReceiver, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("NewOrderReceiver:trans go.msg to c.msg error")
	}
	var receiver C.struct_UltraOrderReceiver
	C.init_order_receiver(cTopic, &receiver)
	return &OrderReceiver{
		receiver: receiver,
		topic:    cTopic,
	}, nil
}

// Receive 从市场接收数据。
// 该方法分配一个指定大小的缓冲区来接收市场数据，
// 并通过C语言接口C.receive_market接收数据。
// 参数:
//
//	bufSize - 指定接收缓冲区的大小。
//
// 返回值:
//
//	接收到的市场数据字符串表示形式。
func (mr *MarketReceiver) MarketReceive(bufSize int) string {
	buffer := make([]byte, bufSize)
	C.receive_market(&mr.receiver, (*C.char)(unsafe.Pointer(&buffer[0])))
	return C.GoString((*C.char)(unsafe.Pointer(&buffer[0])))
}
func (or *OrderReceiver) OrderReceive(bufSize int) string {
	buffer := make([]byte, bufSize)
	C.receive_order(&or.receiver, (*C.char)(unsafe.Pointer(&buffer[0])))
	return C.GoString((*C.char)(unsafe.Pointer(&buffer[0])))
}

func (mr *MarketReceiver) Close() {
	C.free(unsafe.Pointer(mr.topic))
}
func (or *OrderReceiver) Close() {
	C.free(unsafe.Pointer(or.topic))
}

/* 调用方式
marketPublisher, err := util.NewMarketPublisher("market_topic")
	if err != nil {
		log.Fatalf("Failed to create market publisher: %v", err)
	}
	defer marketPublisher.Close()

	err = marketPublisher.MarketPublish("Hello, Market!")
	if err != nil {
		log.Fatalf("Failed to publish market message: %v", err)
	}

	marketReceiver, err := util.NewMarketReceiver("market_topic")
	if err != nil {
		log.Fatalf("Failed to create market receiver: %v", err)
	}
	defer marketReceiver.Close()

	message := marketReceiver.MarketReceive(1024)
	fmt.Println("Received market message:", message)
*/
