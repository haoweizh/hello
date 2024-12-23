package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	//"github.com/coder/websocket"
	"github.com/gorilla/websocket"
	"hello/util"
	"net/http"
	"time"
)

type OrderHandler func(order *Order)
type MsgHandler func(market string, conn *WSConn, message []byte)
type AccountMsgHandler func(market, key string, message []byte)
type SubscribeHandler func(market string, connection *WSConn, subscribes []interface{}) error

type WSConn struct {
	Conn *websocket.Conn
	//默认0 使用ws  1 使用特殊通道market 2使用特殊通道order
	useType          int
	MarketPublisher  *util.MarketPublisher
	MarketSubscriber []byte
	MarketReceiver   *util.MarketReceiver
	OrderPublisher   *util.OrderPublisher
	OrderReceiver    *util.OrderReceiver
}

func (wsConn *WSConn) Close() error {
	if wsConn.useType == 0 {
		err := wsConn.Conn.Close()
		if err != nil {
			util.Log(util.LogLevelError, `close conn err `+err.Error())
			return err
		}
	}
	return nil
}

var lockMap sync.Map // market - *sync.Mutex

func (wsConn *WSConn) WriteMsg(msg []byte) (err error) {
	//ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	//defer cancel()
	value, _ := lockMap.Load(wsConn)
	var connLock *sync.Mutex
	if value == nil {
		connLock = &sync.Mutex{}
		lockMap.Store(wsConn, connLock)
	} else {
		connLock = value.(*sync.Mutex)
	}
	connLock.Lock()
	defer connLock.Unlock()
	if wsConn.Conn == nil {
		return fmt.Errorf(`nil Conn`)
	}
	if wsConn.useType == 0 {
		err = wsConn.Conn.WriteMessage(websocket.TextMessage, msg)
	} else if wsConn.useType == 1 {
		wsConn.MarketPublisher.PublishMarket(string(msg))
		wsConn.MarketSubscriber = msg
	} else if wsConn.useType == 2 {
		wsConn.OrderPublisher.PublishOrder(string(msg))
	}
	//return wsConn.Conn.Write(ctx, websocket.MessageText, msg)
	return
}

func (wsConn *WSConn) WriteJson(body map[string]interface{}) (err error) {
	jsonData, _ := json.Marshal(body)
	return wsConn.WriteMsg(jsonData)
}

//	func newWsCoder(url string) (conn *WSConn, err error) {
//		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
//		defer cancel()
//		c, _, dialErr := websocket.Dial(ctx, url, &websocket.DialOptions{})
//		if dialErr == nil {
//			return &WSConn{Conn: c}, nil
//		}
//		return nil, dialErr
//	}
//
// initChannel 初始化一个通道，根据给定的URL、市场类型和使用类型来选择合适的通道类型。
// 这个函数主要目的是根据不同的市场和使用类型，选择不同的WebSocket连接方式。
// 参数:
//
//	url - 要连接的WebSocket URL。
//	market - 市场类型，例如BinanceSpot、BinancePerp等。
//	useType - 使用类型，根据此参数和AppConfig.UseType的值决定使用哪种连接方式。1为market 2为order
//
// 返回值:
//
//	*WSConn - 成功时返回一个WebSocket连接指针。
//	error - 如果初始化过程中遇到任何问题，则返回错误。
func initChannel(url, market string, useType int) (*WSConn, error) {
	if AppConfig.UseType == "1" {
		switch market {
		case BinanceSpot, BinancePerp, BinanceMargin:
			return newTsChannel(url, "bf", useType)
		default:
			return newWsGorillaChannel(url)
		}
	} else {
		return newWsGorillaChannel(url)
	}
}

// newTsChannel 创建一个新的交易或市场数据通道。
// 参数:
//
//	url - 用于连接的URL，如果为market，则创建市场数据通道，否则创建交易数据通道。
//	tsCode - 用于标识特定交易或市场数据的代码。
//	useType -用于判断是market单还是order单，market为1 order为2
//
// 返回值:
//
//	*WSConn - 一个指向WSConn对象的指针，该对象代表创建的通道。
//	error - 如果创建过程中发生错误，则返回该错误。
func newTsChannel(url, tsCode string, useType int) (*WSConn, error) {
	if useType == 1 {
		marketPublisher, err := util.InitMarketPublisher(tsCode + "_m_sub")
		if err != nil {
			return nil, err
		}
		marketReceiver, err := util.InitMarketReceiver(tsCode + "_m_pub")
		if err != nil {
			return nil, err
		}
		return &WSConn{
			Conn:            nil,
			useType:         useType,
			MarketPublisher: marketPublisher,
			MarketReceiver:  marketReceiver,
		}, nil
	} else if useType == 2 {
		orderPublisher, err := util.InitOrderPublisher(tsCode + "_order_sub")
		if err != nil {
			return nil, err
		}
		orderReceiver, err := util.InitOrderReceiver(tsCode + "_order_pub")
		if err != nil {
			return nil, err
		}
		return &WSConn{
			Conn:           nil,
			useType:        useType,
			OrderPublisher: orderPublisher,
			OrderReceiver:  orderReceiver,
		}, nil
	}
	return nil, errors.New(fmt.Sprintf("url %s not support %s Init Publisher or Receiver", url, tsCode))
}
func newWsGorillaChannel(url string) (*WSConn, error) {
	var connErr error
	var c *websocket.Conn
	util.Log(util.LogLevelInfo, "try to connect "+url)
	dialer := &websocket.Dialer{
		Proxy:          http.ProxyFromEnvironment,
		ReadBufferSize: 1024 * 32,
	}
	c, _, connErr = dialer.Dial(url, nil)
	if connErr == nil {
		if c != nil {
			c.EnableWriteCompression(true)
			c.SetReadLimit(1024 * 1024 * 128)
			c.SetPingHandler(func(appData string) error {
				//util.Log(util.LogLevelInfo, fmt.Sprintf(`%s ping received %s`, url, appData))
				return c.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second*10))
			})
		}
	} else {
		util.Log(util.LogLevelError, `can not create new connection `+connErr.Error())
		if c != nil {
			_ = c.Close()
		}
	}
	if connErr != nil {
		return nil, connErr
	}
	return &WSConn{Conn: c}, nil
}
func chanHandler(market string, stopChan chan struct{}, connection *WSConn, msgHandler MsgHandler) {
	defer func() {
		//err := connection.Conn.Close(websocket.StatusNormalClosure, "")
		err := connection.Close()
		if err != nil {
			util.Log(util.LogLevelError, fmt.Sprintf(`connection closed %s %s`, market, err.Error()))
		}
	}()
	for {
		select {
		case <-stopChan:
			util.Log(util.LogLevelInfo, "get stop struct, return")
			return
		default:
			if connection.useType == 0 {
				//msgType, message, err := connection.Conn.Read(context.Background())
				msgType, message, err := connection.Conn.ReadMessage()
				if err != nil {
					util.Log(util.LogLevelError, fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
					return
				}
				//if msgType == websocket.MessageText {
				if msgType == websocket.TextMessage {
					msgHandler(market, connection, message)
				}
			} else if connection.useType == 1 {
				buf := make([]byte, 4096)
				msgSize := connection.MarketReceiver.ReceiveMarket(buf)
				if msgSize > 0 {
					if needReconnection(buf[:msgSize]) {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`market %s %s  reconnect`, market, buf[:msgSize]))
						connection.MarketPublisher.PublishMarket(string(connection.MarketSubscriber))
					} else {
						msgHandler(market, connection, buf[:msgSize])
					}
				}
			}
		}
	}
}

func WsAccountClient(market, key, url string, accountMsgHandler AccountMsgHandler) (connection *WSConn, err error) {
	util.Log(util.LogLevelInfo, market+` create account channel `+url)
	connection, err = initChannel(url, market, 2)
	if err != nil {
		util.Log(util.LogLevelError, url+"can not create web socket"+err.Error())
		return nil, err
	}
	go func() {
		defer func() {
			//closeErr := connection.Conn.Close(websocket.StatusNormalClosure, "")
			closeErr := connection.Close()
			if closeErr != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`%s connection closed %s`, url, closeErr.Error()))
			}
		}()
		for {
			if connection.useType == 0 {
				//_, message, readErr := connection.Conn.Read(context.Background())
				_, message, readErr := connection.Conn.ReadMessage()
				if readErr != nil {
					util.DelSyncMap(&AppEnvironment.ConnOrder, market, key)
					util.Log(util.LogLevelError, fmt.Sprintf(`%s %s can not read from account ws: %s`, market, url, readErr.Error()))
					return
				}
				if accountMsgHandler != nil {
					accountMsgHandler(market, key, message)
				}
			} else if connection.useType == 2 {
				buf := make([]byte, 4096)
				msgSize := connection.OrderReceiver.ReceiveOrder(buf)
				if msgSize > 0 {
					if needReconnection(buf[:msgSize]) {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`order %s %s reconnect`, market, buf[:msgSize]))
					} else {
						if accountMsgHandler != nil {
							accountMsgHandler(market, key, buf[:msgSize])
						}
					}
				}
			}
		}
	}()
	return connection, nil
}

func WebSocketClient(market, url string, subscribes []interface{}, subHandler SubscribeHandler,
	msgHandler MsgHandler, step int) (socketMap map[*WSConn]bool, msgChans []chan struct{}, connectErr error) {
	util.Log(util.LogLevelInfo, market+` create depth channel `+url)
	socketMap = make(map[*WSConn]bool)
	msgChans = make([]chan struct{}, 0)
	var stepSubscribes []interface{}
	for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		connection, err := initChannel(url, market, 1)
		if err != nil || connection == nil {
			if err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("can not create web socket %s %s %s", market, url, err.Error()))
			}
			return nil, nil, err
		}
		stopChan := make(chan struct{}, 2)
		go chanHandler(market, stopChan, connection, msgHandler)
		if subHandler != nil {
			_ = subHandler(market, connection, stepSubscribes)
		}
		msgChans = append(msgChans, stopChan)
		socketMap[connection] = true
		time.Sleep(time.Millisecond * 100)
	}
	util.Log(util.LogLevelInfo,
		fmt.Sprintf(`ws client add conns %s sockets %d msgChans %d`, market, len(socketMap), len(msgChans)))
	return
}
func needReconnection(buf []byte) bool {
	if strings.Contains(string(buf), "{\"ctlOp\":\"Reconnection\"}") {
		return true
	}
	return false
}
