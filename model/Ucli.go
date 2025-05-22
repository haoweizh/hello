package model

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lucli_ffi
#include "ucli_ffi.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"hello/util"
	"unsafe"
)

type MarketPublisher struct {
	mPublisher *C.struct_UltraMarketPublisher
}
type MarketReceiver struct {
	mReceiver *C.struct_UltraMarketReceiver
}
type OrderPublisher struct {
	oPublisher *C.struct_UltraOrderPublisher
}
type OrderReceiver struct {
	oReceiver *C.struct_UltraOrderReceiver
}

// InitMarketPublisher 创建并初始化一个新的MarketPublisher实例。
// 参数:
//
//	topic: 用于标识发布者主题的字符串。不能为空。
//
// 返回值:
//
//	*MarketPublisher: 一个指向新创建的MarketPublisher实例的指针。
//	error: 如果内存分配失败或初始化过程中出现错误，返回一个错误。
func InitMarketPublisher(topic string) (*MarketPublisher, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("InitMarketPublisher:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cTopic))
	lenTopic := C.uint(len(topic))
	publisher := C.init_market_publisher(cTopic, lenTopic)
	if publisher == nil {
		return nil, errors.New("InitMarketPublisher: C.init_market_publisher error")
	}
	return &MarketPublisher{mPublisher: publisher}, nil
}

func InitOrderPublisher(topic string) (*OrderPublisher, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("InitOrderPublisher:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cTopic))
	lenTopic := C.uint(len(topic))
	publisher := C.init_order_publisher(cTopic, lenTopic)
	if publisher == nil {
		return nil, errors.New("InitOrderPublisher: C.init_order_publisher error")
	}
	return &OrderPublisher{oPublisher: publisher}, nil
}

// SubRecover
// 只有gate需要传入marketType
func SubRecover(market, marketType string, channelType ChannelType) {
	topic := ``
	switch market {
	case BinanceSpot:
		topic = `bs`
	case BinancePerp:
		topic = `bf`
	case Gate:
		if marketType == MarketTypePerp {
			topic = `gf`
		} else if marketType == MarketTypeSpot {
			topic = `gs`
		}
	case OKEX:
		topic = `ok`
	}
	msg := `recovery`
	cMsg := C.CString(`recovery`)
	defer C.free(unsafe.Pointer(cMsg))
	lenTopicMsg := C.uint(len(msg))
	if channelType == ChanTypeMarket {
		topic = topic + `_m_sub-recovery`
		cTopic := C.CString(topic)
		defer C.free(unsafe.Pointer(cTopic))
		lenTopic := C.uint(len(topic))
		publisher := C.init_market_publisher(cTopic, lenTopic)
		if publisher != nil {
			util.Log(util.LogLevelLocal, fmt.Sprintf("publish market %s %s %d %s",
				market, marketType, channelType, msg))
			C.publish_market(publisher, cMsg, lenTopic)
			util.Log(util.LogLevelLocal, fmt.Sprintf("publish market done %s %s %d %s",
				market, marketType, channelType, msg))
		}
	} else if channelType == ChanTypeOrder {
		topic = topic + `_order_sub-recovery`
		cTopic := C.CString(topic)
		defer C.free(unsafe.Pointer(cTopic))
		lenTopic := C.uint(len(topic))
		publisher := C.init_order_publisher(cTopic, lenTopic)
		if publisher != nil {
			C.publish_order(publisher, cMsg, lenTopicMsg)
		}
	}
}

func InitMarketReceiver(topic string) (*MarketReceiver, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("InitMarketReceiver:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cTopic))
	lenTopic := C.uint(len(topic))
	receiver := C.init_market_receiver(cTopic, lenTopic)
	if receiver == nil {
		return nil, errors.New("InitMarketReceiver: C.init_market_receiver error")
	}
	return &MarketReceiver{mReceiver: receiver}, nil
}

func InitOrderReceiver(topic string) (*OrderReceiver, error) {
	cTopic := C.CString(topic)
	if cTopic == nil {
		return nil, errors.New("InitOrderReceiver:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cTopic))
	lenTopic := C.uint(len(topic))
	receiver := C.init_order_receiver(cTopic, lenTopic)
	if receiver == nil {
		return nil, errors.New("InitOrderReceiver: C.init_order_receiver error")
	}
	return &OrderReceiver{oReceiver: receiver}, nil
}

// PublishMarket 发布消息到市场。
// 参数 msg 是要发布的消息内容。
// 返回值 error 用于指示消息发布过程中是否遇到了错误。
// 该函数首先将 Go 字符串转换为 C 字符串，以供 C 语言库使用。
// 如果转换失败（例如，由于内存分配问题），则返回错误。
// 在发布消息之前，检查 C 字符串是否成功分配了内存，
// 确保在使用后释放内存以避免内存泄漏。
func (mp *MarketPublisher) PublishMarket(msg string) error {
	cMsg := C.CString(msg)
	if cMsg == nil {
		return errors.New("PublishMarket:trans go.msg to c.msg error")
	}
	//util.Log(util.LogLevelLocal, fmt.Sprintf("publish market %s", msg))
	defer C.free(unsafe.Pointer(cMsg))
	lenTopic := C.uint(len(msg))
	C.publish_market(mp.mPublisher, cMsg, lenTopic)
	return nil
}
func (op *OrderPublisher) PublishOrder(msg string) error {
	cMsg := C.CString(msg)
	if cMsg == nil {
		return errors.New("PublishOrder:trans go.msg to c.msg error")
	}
	defer C.free(unsafe.Pointer(cMsg))
	lenTopic := C.uint(len(msg))
	C.publish_order(op.oPublisher, cMsg, lenTopic)
	return nil
}

// ReceiveMarket 从市场接收数据。
// 该方法分配一个指定大小的缓冲区来接收市场数据，
// 并通过C语言接口C.receive_market接收数据。
// 参数:
//
//	buf - 数据
//
// 返回值:
//
//	接收到的市场数据字符串表示形式。
func (mr *MarketReceiver) ReceiveMarket(buf []byte) uint {
	cBuf := (*C.char)(unsafe.Pointer(&buf[0]))
	return uint(C.receive_market(mr.mReceiver, cBuf))
}

func (or *OrderReceiver) ReceiveOrder(buf []byte) uint {
	cBuf := (*C.char)(unsafe.Pointer(&buf[0]))
	return uint(C.receive_order(or.oReceiver, cBuf))
}
