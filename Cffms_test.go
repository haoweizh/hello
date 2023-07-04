package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms"
	"hello/cfmms/batch_request/batch_request_for_uniswap_v2"
	"log"
	"math/big"
	"testing"
)

const (
	expBase = 10
)

func Test_Cfmms(t *testing.T) {
	//cfmms.Cmd()
	//cfmms.GetAbiFromEtherscan("./cfmms/contracts_helper.json", "./cfmms/abi/")
	//cfmms.GenerateGoFilesByAbi("./cfmms/abi", "cfmms/abi_go/")
	cfmms.GenerateDeploymentGoFile([]string{"./contracts/uniswap_v2", "./contracts/uniswap_v3"}, "cfmms/deployment/")
}

func Test_Get_pairs_batch_request(t *testing.T) {

	privateKey := "619ee13c815b29d4384ba48c9abe5b7c5db03f130ca0ee5b0f7a5294ad8bcdad"
	deployerPrivateKey, err := crypto.HexToECDSA(privateKey)
	deployerAddress := crypto.PubkeyToAddress(deployerPrivateKey.PublicKey)

	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to connect to the Ethereum client: %v", err))
	}
	fmt.Println("we have a connection")
	factory := common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f") //uniswapv2
	from := big.NewInt(2638438)
	step := big.NewInt(300)

	// 获取部署者的Nonce
	nonce, err := client.PendingNonceAt(context.Background(), deployerAddress)
	if err != nil {
		log.Fatal(err)
	}

	// 构建部署交易数据
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	auth, _ := bind.NewKeyedTransactorWithChainID(deployerPrivateKey, big.NewInt(1))
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	batch_request_for_uniswap_v2.Get_pairs_batch_request(auth, factory, from, step, client)

}

func Test_Get_pairs_batch_request2(t *testing.T) {
	// 连接以太坊客户端
	client, err := ethclient.Dial("https://eth-mainnet.g.alchemy.com/v2/p6QKOpJrOhTeRZ7OT1ufLKVCsqEoKzMG")
	if err != nil {
		log.Fatal(err)
	}

	//// 部署合约的参数
	//from := big.NewInt(2638438)
	//step := big.NewInt(300)
	//factory := common.HexToAddress("0x5c69bee701ef814a2b6a3edd4b1652cb9cc5aa6f")
	//
	//// 创建ABI编码器
	//abiCoder, err := abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"uint256","name":"from","type":"uint256"},{"internalType":"uint256","name":"step","type":"uint256"},{"internalType":"address","name":"factory","type":"address"}],"stateMutability":"nonpayable","type":"constructor"}]`))
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//// 打包合约参数
	//data, err := abiCoder.Pack("constructor", from, step, factory)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	// 合约部署字节码
	contractDeploymentBytecode := "6080604052600080fdfea2646970667358221220f3a6447f9078da7c7fb7393c482159ae841d8387e36746eac1be1d3ea8849a7964736f6c63430008100033"

	// 构造静态调用消息
	callMsg := ethereum.CallMsg{
		From: common.HexToAddress("0x2D50B886Dd7432585c9E15d8438E2D5f1EF0678D"),
		To:   nil,
		Data: []byte(contractDeploymentBytecode),
		Gas:  0,
	}

	// 执行静态调用
	result, err := client.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		fmt.Println("CallContract error")
		log.Fatal(err)
	}

	// 解码返回值
	// 注意：返回值的具体解码取决于合约的返回类型和编码方式
	// 在这个示例中，假设返回的是动态数组（address[]）
	resultLen := len(result)
	if resultLen < 32 {
		log.Fatal("Invalid result length")
	}

	offset := int(result[31]) + 32
	if offset >= resultLen {
		log.Fatal("Invalid result offset")
	}

	var pairs []common.Address
	for offset < resultLen {
		addr := common.BytesToAddress(result[offset : offset+20])
		pairs = append(pairs, addr)
		offset += 32
	}

	// 打印返回值
	for _, pair := range pairs {
		fmt.Println(pair.Hex())
	}
}

func FloatToTokenAmount(amount float64, decimals int64) *big.Int {
	weiFloat := big.NewFloat(amount)
	decimalsBigFloat := big.NewFloat(0).SetInt(Exp10(decimals))
	amountBig := new(big.Float).Mul(weiFloat, decimalsBigFloat)
	r, _ := amountBig.Int(nil)

	return r
}

// Exp10 ...
func Exp10(n int64) *big.Int {
	return new(big.Int).Exp(big.NewInt(expBase), big.NewInt(n), nil)
}
