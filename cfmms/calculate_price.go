package cfmms

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"hello/cfmms/pool"
	"hello/cfmms/utils"
	"math/big"
)

func CalculatePrice(client *ethclient.Client) {

	v2pool := pool.UniswapV2Pool{
		//Address: common.HexToAddress("0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc"), // weth-usdc
		//Address: common.HexToAddress("0xc5117190248F0405D36d12bE11608aa5FDa1dDe3"), // INU / WETH
		Address: common.HexToAddress("0x87e659605548481C1861E971FE4cfb078a711a77"), // ǝdǝd / WETH

	}

	poolIns, err := v2pool.NewFromAddress(v2pool.Address, client)
	if err != nil {
		fmt.Println("NewFromAddress error")
	}

	v2Pool := poolIns.(*pool.UniswapV2Pool)
	v2Pool.GetPoolData(client)

	price := v2Pool.CalculatePrice(v2Pool.TokenA)
	fmt.Println(price)

}

func CalculatePriceV3(client *ethclient.Client) {

	v3pool := pool.UniswapV3Pool{
		Address: common.HexToAddress("0x88e6a0c2ddd26feeb64f039a2c41296fcb3f5640"), // weth-usdc
	}

	v3poolIns, err := v3pool.NewFromAddress(v3pool.Address, client)
	if err != nil {
		fmt.Println(" v3poolIns NewFromAddress error")
	}

	v3Pool := v3poolIns.(pool.UniswapV3Pool)

	v3Pool.GetPoolData(client)

	pa := v3Pool.CalculatePrice(v3Pool.TokenA)
	pb := v3Pool.CalculatePrice(v3Pool.TokenB)
	fmt.Println(pa)
	fmt.Println(pb)

}

func SimulateSwapV2(client *ethclient.Client) {

	v2pool := pool.UniswapV2Pool{
		Address: common.HexToAddress("0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc"), // weth-usdc
		//Address: common.HexToAddress("0xc5117190248F0405D36d12bE11608aa5FDa1dDe3"), // INU / WETH
		//Address: common.HexToAddress("0x87e659605548481C1861E971FE4cfb078a711a77"), // ǝdǝd / WETH

	}

	poolIns, err := v2pool.NewFromAddress(v2pool.Address, client)
	if err != nil {
		fmt.Println("NewFromAddress error")
	}

	v2Pool := poolIns.(*pool.UniswapV2Pool)
	v2Pool.GetPoolData(client)

	//1000 weth
	weth1000, _ := big.NewInt(0).SetString("16793882496467196319299", 10)
	num := v2Pool.SimulateSwap(v2Pool.TokenB, weth1000) // 1weth 换 usdc

	fmt.Println(num.ToSignificant(7))
	fmt.Println(v2Pool.TokenBDecimals)

	beishu := num.Divide(utils.NewFraction(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(v2Pool.TokenADecimals)), nil), big.NewInt(1)))

	fmt.Println(beishu.ToSignificant(7))

}
