package model

const (
	ClientTopic = "ok-client"
	OkxTopic    = "ok-okex"
)

type OkexAgentMessage struct {
	Message     []byte
	ChannelType ChannelType
}
