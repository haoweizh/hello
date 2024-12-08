hello
# 关于生成.so或者.dylib的命令
```
ucli_ffi.c
ucli_ffi.h
//同目录下使用一下命令
gcc -fPIC -shared -o libucli_ffi.so ucli_ffi.c
gcc -dynamiclib -o libucli_ffi.dylib ucli_ffi.c
```
# 环境搭建

1. rust (for foundry)
2. nodejs (for openzeppelin)
3. solidity (for solidity)
4. geth (for ethereum)
5. prmsym (for ethereum)

# 合约框架

## foundry 文档

https://learnblockchain.cn/docs/foundry/i18n/zh/index.html

## foundry init

```
 forge init --force --no-git
```

## foundry test

```
 forge test -vvvvv  // v越多，输出的信息越详细 最多5个
```

## 通过合约ABI生成go文件

```
   ./abigen --abi=/Users/uuuliu/GolandProjects/Go/hello/contracts/UniswapV3/UniswapV3.abi --pkg=UniswapV3 --out=UniswapV3.go
```

## geth 启动命令

```
    ./geth --mainnet --datadir "/Users/uuuliu/Downloads/work/ethereum/data" --http --http.api="eth,web3,net,debug" --authrpc.vhosts="localhost" --authrpc.jwtsecret=/Users/uuuliu/Downloads/work/ethereum/consensus/prysm/jwt.hex
```

## prysm 启动命令

```
    ./prysm.sh beacon-chain --checkpoint-sync-url=https://sync.invis.tools --genesis-beacon-api-url=https://sync.invis.tools  --execution-endpoint=http://localhost:8551 --jwt-secret=/Users/uuuliu/Downloads/work/ethereum/consensus/prysm/jwt.hex
```
