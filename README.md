hello
# 关于生成.so或者.dylib的命令
```
ucli_ffi.c
ucli_ffi.h
//同目录下使用一下命令
gcc -fPIC -shared -o libucli_ffi.so ucli_ffi.c
gcc -dynamiclib -o libucli_ffi.dylib ucli_ffi.c
```
# linux设置记录 
 - 将so文件和.h文件放到/usr/local/lib/目录下
 - 执行以下命令,确保把动态链接库连接到运行时环境
```
   echo "/usr/local/lib" | sudo tee /etc/ld.so.conf.d/ucli_ffi.conf
   sudo ldconfig
```
 - 如果权限不足可以使用下面命令给予相应的权限
```
 sudo chmod 777 /usr/local/lib/libucli_ffi.so
```
 - 使用命令编译go文件
```
 CGO_CFLAGS="-I/usr/local/lib" CGO_LDFLAGS="-L/usr/local/lib -lucli_ffi" go build -ldflags "-w -s" -o /root/btcCarry/helloworld helloworld.go 
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
