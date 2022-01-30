module hello

go 1.16

require (
	github.com/Kucoin/kucoin-go-sdk v1.2.8
	github.com/Kucoin/kumex-go-sdk v0.0.0-00010101000000-000000000000
	github.com/adshao/go-binance/v2 v2.3.4 // indirect
	github.com/antihax/optional v1.0.0
	github.com/bitly/go-simplejson v0.5.0
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/gateio/gateapi-go/v6 v6.21.6
	github.com/gateio/gatews/go v0.0.0-20210825031544-0516c138bb74
	github.com/gin-gonic/gin v1.7.4
	github.com/gorilla/websocket v1.4.2
	github.com/jinzhu/configor v1.2.1
	github.com/pkg/errors v0.9.1 // indirect
	github.com/satori/go.uuid v1.2.0
	gorm.io/driver/postgres v1.1.0
	gorm.io/gorm v1.21.14
)

replace github.com/Kucoin/kumex-go-sdk => github.com/SonicWW/kucoin-futures-go-sdk v1.0.5 // indirect
