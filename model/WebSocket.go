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

// ChannelType 定义通道类型的枚举
type ChannelType int

const (
	// ChanTypeWS 表示WebSocket相关的通道类型 0
	ChanTypeWS ChannelType = iota
	// ChanTypeMarket 表示市场相关的通道类型 1
	ChanTypeMarket
	// ChanTypeOrder 表示订单相关的通道类型 2
	ChanTypeOrder
)

// String 方法用于将 ChannelType 转换为字符串表示
func (ct ChannelType) String() string {
	switch ct {
	case ChanTypeMarket:
		return "market"
	case ChanTypeOrder:
		return "order"
	case ChanTypeWS:
		return "ws"
	default:
		return "unknown"
	}
}

type WSConn struct {
	conn *websocket.Conn
	//默认ws 使用ws  ChanTypeMarket 使用特殊通道market WSTypeOrder使用特殊通道order
	WSType           ChannelType
	MarketPublisher  *util.MarketPublisher
	MarketSubscriber []byte
	MarketReceiver   *util.MarketReceiver
	OrderPublisher   *util.OrderPublisher
	OrderReceiver    *util.OrderReceiver
}

func (wsConn *WSConn) Close() {
	if wsConn.WSType == ChanTypeWS {
		err := wsConn.conn.Close()
		if err != nil {
			util.Log(util.LogLevelError, `close conn err `+err.Error())
			return
		}
	}
	return
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
	if wsConn.conn == nil {
		return fmt.Errorf(`nil conn`)
	}
	if wsConn.WSType == ChanTypeWS {
		err = wsConn.conn.WriteMessage(websocket.TextMessage, msg)
	} else if wsConn.WSType == ChanTypeMarket {
		err = wsConn.MarketPublisher.PublishMarket(string(msg))
		wsConn.MarketSubscriber = msg
	} else if wsConn.WSType == ChanTypeOrder {
		err = wsConn.OrderPublisher.PublishOrder(string(msg))
	}
	//return wsConn.conn.Write(ctx, websocket.MessageText, msg)
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
//			return &WSConn{conn: c}, nil
//		}
//		return nil, dialErr
//	}

// initChannel 初始化一个通道，根据给定的URL、市场类型和使用类型来选择合适的通道类型。
// 这个函数主要目的是根据不同的市场和使用类型，选择不同的WebSocket连接方式。
// 参数:
//
//	url - 要连接的WebSocket URL。
//	market - 市场类型，例如BinanceSpot、BinancePerp等。
//	WSType - 使用类型，根据此参数和AppConfig.UseType的值决定使用哪种连接方式。1为market 2为order
//
// 返回值:
//
//	*WSConn - 成功时返回一个WebSocket连接指针。
//	error - 如果初始化过程中遇到任何问题，则返回错误。
func initChannel(url, market string, wsType ChannelType) (*WSConn, error) {
	if AppConfig.SpecialChan == "1" {
		switch market {
		case BinanceSpot, BinancePerp, BinanceMargin:
			return newTsChannel(url, "bf", wsType)
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
//	WSType -用于判断是market单还是order单，market为1 order为2
//
// 返回值:
//
//	*WSConn - 一个指向WSConn对象的指针，该对象代表创建的通道。
//	error - 如果创建过程中发生错误，则返回该错误。
func newTsChannel(url, tsCode string, wsType ChannelType) (*WSConn, error) {
	if wsType == ChanTypeMarket {
		marketPublisher, errPub := util.InitMarketPublisher(tsCode + "_m_sub")
		if errPub != nil {
			return nil, errPub
		}
		marketReceiver, errRec := util.InitMarketReceiver(tsCode + "_m_pub")
		if errRec != nil {
			return nil, errRec
		}
		return &WSConn{
			conn:            nil,
			WSType:          wsType,
			MarketPublisher: marketPublisher,
			MarketReceiver:  marketReceiver,
		}, nil
	} else if wsType == ChanTypeOrder {
		orderPublisher, errPub := util.InitOrderPublisher(tsCode + "_order_sub")
		if errPub != nil {
			return nil, errPub
		}
		orderReceiver, errRec := util.InitOrderReceiver(tsCode + "_order_pub")
		if errRec != nil {
			return nil, errRec
		}
		return &WSConn{
			conn:           nil,
			WSType:         wsType,
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
	return &WSConn{conn: c, WSType: ChanTypeWS}, nil
}

func publicHandler(market string, stopChan chan struct{}, connection *WSConn, msgHandler MsgHandler) {
	defer func() {
		//err := connection.conn.Close(websocket.StatusNormalClosure, "")
		connection.Close()
	}()
	for {
		select {
		case <-stopChan:
			util.Log(util.LogLevelInfo, "get stop struct, return")
			return
		default:
			if connection.WSType == ChanTypeWS {
				//msgType, message, err := connection.conn.Read(context.Background())
				msgType, message, err := connection.conn.ReadMessage()
				if err != nil {
					util.Log(util.LogLevelError, fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
					return
				}
				//if msgType == websocket.MessageText {
				if msgType == websocket.TextMessage {
					msgHandler(market, connection, message)
				}
			} else if connection.WSType == ChanTypeMarket {
				buf := make([]byte, 4096)
				msgSize := connection.MarketReceiver.ReceiveMarket(buf)
				if msgSize > 0 {
					if needReconnection(buf[:msgSize]) {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`chan need reconnect market %s %s`, market, buf[:msgSize]))
						err := connection.MarketPublisher.PublishMarket(string(connection.MarketSubscriber))
						if err != nil {
							util.Log(util.LogLevelInfo, fmt.Sprintf(`%s can not publish market %s`, market, err.Error()))
						}
					} else if msgHandler != nil {
						msgHandler(market, connection, buf[:msgSize])
					}
				}
			}
		}
	}
}

func WsPrivateClient(market, key, url string, accountMsgHandler AccountMsgHandler) (connection *WSConn, err error) {
	util.Log(util.LogLevelInfo, market+` create account channel `+url)
	connection, err = initChannel(url, market, ChanTypeOrder)
	if err != nil {
		util.Log(util.LogLevelError, url+"can not create web socket"+err.Error())
		return nil, err
	}
	go func() {
		defer func() {
			//closeErr := connection.conn.Close(websocket.StatusNormalClosure, "")
			connection.Close()
		}()
		for {
			if connection.WSType == ChanTypeWS {
				//_, message, readErr := connection.conn.Read(context.Background())
				_, message, readErr := connection.conn.ReadMessage()
				if readErr != nil {
					value, _ := util.LoadSyncMap(&AppEnvironment.ConnOrder, market, key)
					if value != nil && value == connection.conn {
						util.Log(util.LogLevelError, fmt.Sprintf(`delete connect %s %s %v`, market, key, value))
						util.DelSyncMap(&AppEnvironment.ConnOrder, market, key)
					}
					util.Log(util.LogLevelError, fmt.Sprintf(`%s %s can not read from account ws: %s`, market, url, readErr.Error()))
					return
				}
				if accountMsgHandler != nil {
					accountMsgHandler(market, key, message)
				}
			} else if connection.WSType == ChanTypeOrder {
				buf := make([]byte, 4096)
				msgSize := connection.OrderReceiver.ReceiveOrder(buf)
				if msgSize > 0 {
					if needReconnection(buf[:msgSize]) {
						util.Log(util.LogLevelInfo, fmt.Sprintf(`order %s %s reconnect`, market, buf[:msgSize]))
					} else if accountMsgHandler != nil {
						accountMsgHandler(market, key, buf[:msgSize])
					}
				}
			}
		}
	}()
	return connection, nil
}

func WsPublicClient(market, url string, subscribes []interface{}, subHandler SubscribeHandler,
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
		connection, err := initChannel(url, market, ChanTypeMarket)
		if err != nil || connection == nil {
			if err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf("can not create web socket %s %s %s", market, url, err.Error()))
			}
			return nil, nil, err
		}
		stopChan := make(chan struct{}, 2)
		go publicHandler(market, stopChan, connection, msgHandler)
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
