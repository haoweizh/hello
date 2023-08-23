package Entity

type Chain struct {
	Name         string
	LocalBlockID int64
}

type Wallet struct {
	ChainName  string
	Address    string
	PubKey     string
	PrivateKey string
	Coins      []string
	Balances   map[string]float64
}

// todo 各个chain的读写功能。可能包括account balance查询、合约调用、block信息查询等

func (chain *Chain) RefreshBalance(wallet *Wallet) {

}

func (chain *Chain) InvokeContract(contractAddress string, wallet *Wallet) {

}

func (chain *Chain) LoadFromDB() {

}

//4. Tick: 包括market, coinLeft, coinRight, time(用于表明tick新鲜度), []bid, []ask等
//5. TickPool: 提供tick检索获取、写入更新功能
//6. Ring: 可获利交易环
//7. Setting: 同现有setting设计，用于限制搬砖的范围
