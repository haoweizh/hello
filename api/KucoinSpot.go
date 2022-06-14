package api

import (
	"github.com/Kucoin/kucoin-go-sdk"
	"hello/model"
)

func kucoinRelatedClient(key, secret, passPhrase string) *kucoin.ApiService {
	if key == "" || secret == "" || passPhrase == "" {
		key = model.AppConfig.KucoinRelatedKey
		secret = model.AppConfig.KucoinRelatedSecret
		passPhrase = model.AppConfig.Phase
	}
	client := kucoin.NewApiService(
		kucoin.ApiKeyOption(key),
		kucoin.ApiSecretOption(secret),
		kucoin.ApiPassPhraseOption(passPhrase),
		kucoin.ApiKeyVersionOption("2"))
	return client
}

func getMarketsKucoinSpot(key, secret string) (marketInfos map[string]*model.MarketInfo) {
	marketInfos = make(map[string]*model.MarketInfo)
	appendRelatedMarketsKucoin(key, marketInfos)
	return marketInfos
}
