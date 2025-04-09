package model

import (
	"encoding/json"
	"errors"
	"fmt"
	gateWs "github.com/gateio/gatews/go"
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
	conn   *websocket.Conn
	Closed bool
	WSChan chan []byte
	//默认ws 使用ws  ChanTypeMarket 使用特殊通道market WSTypeOrder使用特殊通道order
	WSType           ChannelType
	MarketPublisher  *MarketPublisher
	MarketSubscriber []byte
	MarketReceiver   *MarketReceiver
	OrderPublisher   *OrderPublisher
	OrderReceiver    *OrderReceiver
}

func (wsConn *WSConn) Close() {
	if wsConn.WSType == ChanTypeWS {
		wsConn.Closed = true
		util.Log(util.LogLevelInfo, fmt.Sprintf("close websocket connection %s", wsConn.WSType.String()))
		//close(wsConn.WSChan)
		err := wsConn.conn.Close()
		if err != nil {
			util.Log(util.LogLevelError, `close conn err `+err.Error())
			return
		}
	}
	return
}

func (wsConn *WSConn) handle() {
	for !util.Terminal {
		msg := <-wsConn.WSChan
		if len(wsConn.WSChan) > 10 {
			util.Log(util.LogLevelError, fmt.Sprintf(`wsConn wait list 10 %d %s %#v`, len(wsConn.WSChan), string(msg), wsConn))
			continue
		}
		//if strings.Contains(string(msg), `order`) {
		//	util.Log(util.LogLevelInfo, fmt.Sprintf("time mark after %s %d %s", wsConn.WSType.String(), time.Now().UnixMicro(), string(msg)))
		//}
		var err error
		if wsConn.WSType == ChanTypeWS {
			err = wsConn.conn.WriteMessage(websocket.TextMessage, msg)
		} else if wsConn.WSType == ChanTypeMarket {
			if len(string(msg)) <= 8000 {
				err = wsConn.MarketPublisher.PublishMarket(string(msg))
				wsConn.MarketSubscriber = msg
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf("too big msg %s %d", string(msg), len(msg)))
			}
		} else if wsConn.WSType == ChanTypeOrder {
			if len(string(msg)) <= 8000 {
				err = wsConn.OrderPublisher.PublishOrder(string(msg))
			} else {
				util.Log(util.LogLevelInfo, fmt.Sprintf("too big msg order %s %d", string(msg), len(msg)))
			}
		}
		if err != nil {
			wsConn.Closed = true
			util.Log(util.LogLevelError, `handle ws err `+err.Error())
		}
	}
	fmt.Println(fmt.Sprintf(`ws private exit write`))
}

func (wsConn *WSConn) WriteMsg(msg []byte) (err error) {
	if (wsConn.conn == nil || wsConn.Closed) && wsConn.WSType == ChanTypeWS {
		return fmt.Errorf(fmt.Sprintf(`nil conn %s`, wsConn.WSType.String()))
	}
	//if strings.Contains(string(msg), `order`) {
	//	util.Log(util.LogLevelInfo, fmt.Sprintf("time mark before %d %s", time.Now().UnixMicro(), string(msg)))
	//}
	wsConn.WSChan <- msg
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
func initChannel(account *Account, url, market string, wsType ChannelType, noSpecialChan bool) (newCreate bool, wsConn *WSConn, err error) {
	if account == nil || account.Index == 0 {
		switch market {
		case BinancePerp:
			if noSpecialChan || AppConfig.Binancechan == 0 {
				return newWsGorillaChannel(url)
			}
			return newTsChannel(url, "bf", wsType)
		case BinanceSpot:
			if noSpecialChan || AppConfig.Binancechan == 0 {
				return newWsGorillaChannel(url)
			}
			return newTsChannel(url, "bs", wsType)
		case Gate:
			if noSpecialChan || AppConfig.Gatechan == 0 {
				return newWsGorillaChannel(url)
			} else if url == gateWs.FuturesUsdtUrl {
				return newTsChannel(url, "gf", wsType)
			} else if url == gateWs.BaseUrl {
				return newTsChannel(url, "gs", wsType)
			}
			return newWsGorillaChannel(url)
		case OKEX:
			if noSpecialChan || AppConfig.Okchan == 0 {
				return newWsGorillaChannel(url)
			}
			return newTsChannel(url, "ok", wsType)
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
func newTsChannel(url, tsCode string, wsType ChannelType) (newCreate bool, wsConn *WSConn, err error) {
	value, ok := util.LoadSyncMap(&AppEnvironment.SpecialChans, tsCode, wsType.String())
	if value != nil && ok {
		return false, value.(*WSConn), nil
	}
	if wsType == ChanTypeMarket {
		wsConn = &WSConn{conn: nil, WSType: wsType, WSChan: make(chan []byte, 50)}
		wsConn.MarketPublisher, err = InitMarketPublisher(tsCode + "_m_sub")
		if err != nil {
			return false, nil, err
		}
		wsConn.MarketReceiver, err = InitMarketReceiver(tsCode + "_m_pub")
		if err != nil {
			return false, nil, err
		}
		util.StoreSyncMap(&AppEnvironment.SpecialChans, wsConn, tsCode, wsType.String())
		go wsConn.handle()
		return true, wsConn, nil
	} else if wsType == ChanTypeOrder {
		wsConn = &WSConn{conn: nil, WSType: wsType, WSChan: make(chan []byte, 50)}
		wsConn.OrderPublisher, err = InitOrderPublisher(tsCode + "_order_sub")
		if err != nil {
			return false, nil, err
		}
		wsConn.OrderReceiver, err = InitOrderReceiver(tsCode + "_order_pub")
		if err != nil {
			return false, nil, err
		}
		util.StoreSyncMap(&AppEnvironment.SpecialChans, wsConn, tsCode, wsType.String())
		go wsConn.handle()
		util.Log(util.LogLevelInfo, fmt.Sprintf("new ts channel %s %s _m_sub _m_pub", tsCode, wsType.String()))
		return true, wsConn, nil
	}
	return true, nil, errors.New(fmt.Sprintf("url %s not support %s Init Publisher or Receiver", url, tsCode))
}

func newWsGorillaChannel(url string) (newCreate bool, wsConn *WSConn, err error) {
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
		return true, nil, connErr
	}
	wsConn = &WSConn{conn: c, WSType: ChanTypeWS, WSChan: make(chan []byte, 50)}
	go wsConn.handle()
	return true, wsConn, nil
}

func publicHandler(market, url string, connection *WSConn, subHandler SubscribeHandler, step int, msgHandler MsgHandler) {
	defer func() {
		//err := connection.conn.Close(websocket.StatusNormalClosure, "")
		connection.Close()
	}()
	for !util.Terminal {
		if connection.WSType == ChanTypeWS {
			//msgType, message, err := connection.conn.Read(context.Background())
			msgType, message, err := connection.conn.ReadMessage()
			if err != nil {
				util.Log(util.LogLevelError, fmt.Sprintf(`%s can not read from websocket: %s`, market, err.Error()))
				return
			}
			//if msgType == websocket.MessageText {
			if msgType == websocket.TextMessage {
				go msgHandler(market, connection, message)
			}
		} else if connection.WSType == ChanTypeMarket {
			buf := make([]byte, 4096)
			msgSize := connection.MarketReceiver.ReceiveMarket(buf)
			if msgSize > 0 {
				if NeedReconnection(buf[:msgSize]) {
					value, _ := AppEnvironment.PubSubscribes.Load(fmt.Sprintf("%s*%s", market, url))
					if value != nil {
						subscribes := value.([]interface{})
						var stepSubscribes []interface{}
						for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
							if (i+1)*step < len(subscribes) {
								stepSubscribes = subscribes[i*step : (i+1)*step]
							} else {
								stepSubscribes = subscribes[i*step:]
							}
							_ = subHandler(market, connection, stepSubscribes)
							util.Log(util.LogLevelLocal, fmt.Sprintf(`chan need reconnect market %s %s sub %#v`,
								market, buf[:msgSize], stepSubscribes))
							time.Sleep(time.Millisecond * 200)
						}
					}
				} else if msgHandler != nil {
					go msgHandler(market, connection, buf[:msgSize])
				}
			}
		}
	}
}

func WsPrivateClient(account *Account, connMap *sync.Map, connKey, market, url string, accountMsgHandler AccountMsgHandler,
	noSpecialChan bool) (connection *WSConn, err error) {
	util.Log(util.LogLevelInfo, fmt.Sprintf(` create account channel %s %d %s`, market, account.Index, url))
	newCreate := false
	newCreate, connection, err = initChannel(account, url, market, ChanTypeOrder, noSpecialChan)
	if !newCreate {
		return connection, err
	}
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
				_, message, readErr := connection.conn.ReadMessage()
				if readErr != nil {
					value, _ := connMap.Load(connKey)
					if value != nil && value == connection.conn {
						util.Log(util.LogLevelError, fmt.Sprintf(`delete connect %s %v`, connKey, value))
						connMap.Delete(connKey)
					}
					util.Log(util.LogLevelError, fmt.Sprintf(`%s %s can not read from account ws: %s`, market, url, readErr.Error()))
					return
				}
				if accountMsgHandler != nil {
					go accountMsgHandler(market, account.Key, message)
				}
			} else if connection.WSType == ChanTypeOrder {
				buf := make([]byte, 8192)
				msgSize := connection.OrderReceiver.ReceiveOrder(buf)
				if msgSize > 0 {
					if NeedReconnection(buf[:msgSize]) {
						util.Log(util.LogLevelLocal, fmt.Sprintf(`order channel reconnect %s %s `, market, buf[:msgSize]))
					}
					if accountMsgHandler != nil {
						go accountMsgHandler(market, account.Key, buf[:msgSize])
					}
				}
			}
		}
		//fmt.Println(fmt.Sprintf(`ws private exit resd %d %s`, account.Index, account.Market))
	}()
	return connection, nil
}

var pubHandleSettle sync.Map // url - bool

func WsPublicClient(market, url string, subscribes []interface{}, subHandler SubscribeHandler,
	msgHandler MsgHandler, step int, noSpecialChan bool) (socketMap map[*WSConn]bool, connectErr error) {
	util.Log(util.LogLevelLocal, fmt.Sprintf(`create depth channel WsPublicClient %s %d %s`, market, len(subscribes), url))
	AppEnvironment.PubSubscribes.Store(fmt.Sprintf("%s*%s", market, url), subscribes)
	socketMap = make(map[*WSConn]bool)
	var stepSubscribes []interface{}
	for i := 0; subscribes != nil && i*step < len(subscribes); i++ {
		if (i+1)*step < len(subscribes) {
			stepSubscribes = subscribes[i*step : (i+1)*step]
		} else {
			stepSubscribes = subscribes[i*step:]
		}
		newCreate, connection, err := initChannel(nil, url, market, ChanTypeMarket, noSpecialChan)
		if err != nil || connection == nil {
			util.Log(util.LogLevelLocal, fmt.Sprintf("can not create web socket %s %s %#v", market, url, err))
			return nil, err
		}
		go func() {
			settle, ok := pubHandleSettle.Load(url)
			_ = subHandler(market, connection, stepSubscribes)
			if newCreate || !ok || settle.(bool) == false {
				pubHandleSettle.Store(url, true)
				util.Log(util.LogLevelInfo, fmt.Sprintf("new create public chan %s %s", market, url))
				publicHandler(market, url, connection, subHandler, step, msgHandler)
			}
		}()
		socketMap[connection] = true
		time.Sleep(time.Millisecond * 200)
	}
	util.Log(util.LogLevelInfo,
		fmt.Sprintf(`ws client add conns %s sockets %d`, market, len(socketMap)))
	return
}

func NeedReconnection(buf []byte) bool {
	if strings.Contains(string(buf), "{\"ctlOp\":\"Reconnection\"}") {
		return true
	}
	return false
}

func InitConn(tsCode string, wsType ChannelType) (newCreate bool, wsConn *WSConn, err error) {
	return newTsChannel("", tsCode, wsType)
}
