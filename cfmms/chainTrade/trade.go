package chainTrade

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"
	"log"
	"math/big"
)

// anvil 生成测试地址

//(0) "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" (300.000000000000000000 ETH)
//(1) "0x70997970C51812dc3A010C7d01b50e0d17dc79C8" (300.000000000000000000 ETH)
//(2) "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC" (300.000000000000000000 ETH)
//(3) "0x90F79bf6EB2c4f870365E785982E1f101E93b906" (300.000000000000000000 ETH)
//(4) "0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65" (300.000000000000000000 ETH)
//
//Private Keys
//==================
//
//(0) 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
//(1) 0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d
//(2) 0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a
//(3) 0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6
//(4) 0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a

// const ChainClientRpc = "http://127.0.0.1:8545"
const ChainClientRpc = "https://goerli.infura.io/v3/bc26f6bef4a34dd586cb012f82d561d5"

var client *ethclient.Client

func init() {

	var err error
	client, err = ethclient.Dial(ChainClientRpc)
	if err != nil {
		log.Fatal(err)
	}

}

// 生成一个钱包地址
func GenerateAccount() string {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)

	// 钱包私钥
	fmt.Println("钱包私钥: ", hexutil.Encode(privateKeyBytes)[2:]) // 0xfad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	fmt.Println(hexutil.Encode(publicKeyBytes)[4:]) // 0x049a7df67f79246283fdc93af76d4f8cdd62c4886e8cd870944e817dd0b97934fdd7719d0810951e03418205868a5c1b40b192451367f28e0088dd75e15de40c05

	// 地址方法1
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Println("钱包地址: ", address) // 0x96216849c49358B10257cb55b28eA603c874b05E

	// 地址方法2
	hash := sha3.NewLegacyKeccak256()
	hash.Write(publicKeyBytes[1:])
	fmt.Println(hexutil.Encode(hash.Sum(nil)[12:])) // 0x96216849c49358b10257cb55b28ea603c874b05e
	return hexutil.Encode(hash.Sum(nil)[12:])       // 0x96
}

// 查询余额
func GetBalance(address common.Address) *big.Int {

	balance, _ := client.BalanceAt(context.Background(), address, nil)

	return balance
}

// 减去pending的余额
func GetPendingBalance(account common.Address) *big.Int {
	pendingBalance, err := client.PendingBalanceAt(context.Background(), account)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pendingBalance) // 25729324269165216042
	return pendingBalance
}

// 查询区块头

func GetBlockHeader() *types.Header {
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}

	result, err := json.Marshal(header)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println(string(result))
	return header
}

// 查询完整区块

func GetBlock(blockNumber *big.Int) *types.Block {
	block, err := client.BlockByNumber(context.Background(), blockNumber)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(block.TxHash().Hex())
	fmt.Println(block.ParentHash())

	result, err := json.Marshal(block)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println(string(result))
	return block
}

// 查询交易
func GetTransaction(hash common.Hash) *types.Transaction {
	transaction, isPending, err := client.TransactionByHash(context.Background(), hash)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(isPending)

	result, err := json.Marshal(transaction)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println(string(result))
	return transaction
}

// 查询当前交易头内区块交易数量
func GetTransactionCount(block *types.Block) uint {
	count, err := client.TransactionCount(context.Background(), block.Hash())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(count)
	return count
}

// 查询当前交易头内区块交易数据
func GetTransactionInfo(block *types.Block) {
	// TODO: 如何获取sender地址
	for _, tx := range block.Transactions() {
		fmt.Println(tx.Hash().Hex())        // 0x5d49fcaa394c97ec8a9c3e7bd9e8388d420fb050a52083ca52ff24b3b65bc9c2
		fmt.Println(tx.Value().String())    // 10000000000000000
		fmt.Println(tx.Gas())               // 105000
		fmt.Println(tx.GasPrice().Uint64()) // 102000000000
		fmt.Println(tx.Nonce())             // 110644
		fmt.Println(tx.Data())              // []
		fmt.Println(tx.To().Hex())          // 0x55fE59D8Ad77035154dDd0AD0388D09Dd4047A8e
	}
}

func CreateUnlockEvent(address string) {

}

func BuyFormUniswapV2(amount float64, decimals int64) {

}
