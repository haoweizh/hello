package service

import (
	"fmt"
	"hello/api"
	"hello/model"
	"hello/util"
	"math"
	"strconv"
)

// OkexAgentService 处理 WebSocket 消息的 Service
type OkexAgentService struct {
	ClientConn   *model.WSConn
	OkexConn     *model.WSConn
	ClientToOkex chan model.OkexAgentMessage
	OkexToClient chan model.OkexAgentMessage
	doneCh       chan struct{}
}

// NewOkexAgentService 创建一个新的 WebSocketService 实例
func NewOkexAgentService(conn *model.WSConn, okexConn *model.WSConn) *OkexAgentService {
	return &OkexAgentService{
		ClientConn:   conn,
		OkexConn:     okexConn,
		ClientToOkex: make(chan model.OkexAgentMessage, 10000),
		OkexToClient: make(chan model.OkexAgentMessage, 10000),
		doneCh:       make(chan struct{}),
	}
}

// HandleClientPublicMessages 处理公有消息
func (s *OkexAgentService) HandleClientPublicMessages() {
	defer close(s.doneCh)
	go func() {
		for !util.Terminal {
			buf := make([]byte, 4096)
			msgSize := s.ClientConn.MarketReceiver.ReceiveMarket(buf)
			if msgSize > 0 {
				util.Log(util.LogLevelInfo, "ClientConn rev msg :"+string(buf[:msgSize]))
				select {
				case s.ClientToOkex <- s.processMessage(buf[:msgSize], model.ChanTypeMarket):
				case <-s.doneCh:
					return
				}
			}
		}
	}()
	go func() {
		for !util.Terminal {
			buf := make([]byte, 4096)
			msgSize := s.OkexConn.MarketReceiver.ReceiveMarket(buf)
			if msgSize > 0 {
				util.Log(util.LogLevelInfo, "OkexConn rev msg :"+string(buf[:msgSize]))
				// 将消息放入通道
				select {
				case s.OkexToClient <- s.processMessage(buf[:msgSize], model.ChanTypeMarket):
				case <-s.doneCh:
					return
				}
			}
		}
	}()
}

// HandleClientPrivateMessages 处理私有消息
func (s *OkexAgentService) HandleClientPrivateMessages() {
	defer close(s.doneCh)
	go func() {
		for !util.Terminal {
			buf := make([]byte, 4096)
			msgSize := s.ClientConn.OrderReceiver.ReceiveOrder(buf)
			if msgSize > 0 {
				// 将消息放入通道
				select {
				case s.ClientToOkex <- s.processMessage(buf[:msgSize], model.ChanTypeOrder):
				case <-s.doneCh:
					return
				}
			}
		}
	}()
	go func() {
		for !util.Terminal {
			buf := make([]byte, 4096)
			msgSize := s.OkexConn.OrderReceiver.ReceiveOrder(buf)
			if msgSize > 0 {
				// 将消息放入通道
				select {
				case s.OkexToClient <- s.processMessage(buf[:msgSize], model.ChanTypeOrder):
				case <-s.doneCh:
					return
				}
			}
		}
	}()
}

// processMessage 处理接收到的消息
func (s *OkexAgentService) processMessage(message []byte, wsType model.ChannelType) model.OkexAgentMessage {
	agentMessage := model.OkexAgentMessage{Message: message, ChannelType: wsType}
	return agentMessage
}

// HandleMessages 处理通道中的消息
func (s *OkexAgentService) HandleMessages() {
	for !util.Terminal {
		select {
		case msg, ok := <-s.ClientToOkex:
			if !ok {
				return
			}
			if msg.ChannelType == model.ChanTypeMarket {
				util.Log(util.LogLevelInfo, "HandleMessages rev msg :"+string(msg.Message))
				if model.NeedReconnection(msg.Message) || s.ClientConn == nil {
					_, clientMarketConn, err := model.InitConn(model.ClientTopic, model.ChanTypeMarket)
					if err != nil {
						s.ClientConn = clientMarketConn
					}
				}
				err := s.OkexConn.MarketPublisher.PublishMarket(string(msg.Message))
				if err != nil {
					util.Log(util.LogLevelError, "okex public write error:"+err.Error())

				}
			} else if msg.ChannelType == model.ChanTypeOrder {
				err := s.OkexConn.OrderPublisher.PublishOrder(string(msg.Message))
				if err != nil {
					util.Log(util.LogLevelError, "okex private write error:"+err.Error())
				}
			}
		case msg, ok := <-s.OkexToClient:
			if !ok {
				return
			}
			if msg.ChannelType == model.ChanTypeMarket {
				if model.NeedReconnection(msg.Message) || s.OkexConn == nil {
					_, okexMarketConn, err := model.InitConn(model.OkxTopic, model.ChanTypeMarket)
					if err != nil {
						s.OkexConn = okexMarketConn
					}
				}
				//处理message
				newMessage := Process(msg.Message)
				if newMessage != nil {
					err := s.ClientConn.MarketPublisher.PublishMarket(string(newMessage))
					if err != nil {
						util.Log(util.LogLevelError, "client public write error:"+err.Error())
					}
				}
			} else if msg.ChannelType == model.ChanTypeOrder {
				err := s.ClientConn.OrderPublisher.PublishOrder(string(msg.Message))
				if err != nil {
					util.Log(util.LogLevelError, "client private write error:"+err.Error())
				}
			}
		}
	}
}

func Process(event []byte) []byte {
	responseJson, err := util.NewJSON(event)
	if err != nil || responseJson == nil || responseJson.Get(`data`) == nil ||
		len(responseJson.Get(`data`).MustArray()) == 0 ||
		responseJson.GetPath(`arg`, `instId`) == nil {
		return event
	}
	channel := responseJson.GetPath("arg", "channel").MustString()
	instId := responseJson.GetPath("arg", "instId").MustString()
	//data 数组
	data := responseJson.Get(`data`).MustArray()[0].(map[string]interface{})
	//是否存在旧的data数据
	lastData, ok := util.LoadSyncMap(&model.AppEnvironment.OkexPubMarkets, channel, instId)
	//不存在就记录并且返回当前数据。
	if !ok || lastData == nil {
		util.StoreSyncMap(&model.AppEnvironment.OkexPubMarkets, data, channel, instId)
		return event
	}
	//存在 就判断不同的channel
	switch channel {
	case "bbo-tbt":
		bidAsk := api.HandleBooksOKEX(instId, data)
		lastBidAsk := api.HandleBooksOKEX(instId, lastData.(map[string]interface{}))
		//时间戳差值小于阈值，则不更新
		if bidAsk.Ts-lastBidAsk.Ts < model.AppConfig.TimeThreshold {
			return nil
		} else {
			//ask 1和bid 1 的变化大于阈值就发送新的消息
			if isPriceChangeBig(bidAsk.Asks[0].Price, lastBidAsk.Asks[0].Price) || isPriceChangeBig(bidAsk.Bids[0].Price, lastBidAsk.Bids[0].Price) {
				util.StoreSyncMap(&model.AppEnvironment.OkexPubMarkets, data, channel, instId)
				return event
			}
			return nil
		}
	case "funding-rate":
		newTs, _ := strconv.ParseInt(data[`ts`].(string), 10, 64)
		oldTs, _ := strconv.ParseInt(lastData.(map[string]interface{})[`ts`].(string), 10, 64)
		if newTs-oldTs < int64(model.AppConfig.FundingTimeThreshold*1000) {
			return nil
		} else {
			util.StoreSyncMap(&model.AppEnvironment.OkexPubMarkets, data, channel, instId)
			return event
		}
	}
	return event
}

// 测算价格
func isPriceChangeBig(currentPrice, lastPrice float64) bool {
	if currentPrice == 0 {
		return false // 避免除以零的情况
	}
	priceChange := math.Abs(currentPrice - lastPrice)
	percentageChange := priceChange / currentPrice
	util.Log(util.LogLevelInfo, fmt.Sprintf("currentPrice: %f, lastPrice: %f, priceChange: %f, percentageChange: %f", currentPrice, lastPrice, priceChange, percentageChange))
	return percentageChange > model.AppConfig.PriceThreshold
}

// Close 关闭服务
func (s *OkexAgentService) Close() {
	close(s.doneCh)
}
