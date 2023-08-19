hello
(注： CEX相关兼容性改造工作未在本文中进行描述)

# Entities
1. Chain: 分为DriverEthereum/DriverBSC等，封装了各个chain的读写功能。可能包括account balance查询、合约调用、block信息查询等
注： Chain的信息可以存DB，这样程序重启后从DB中可以获取同步了多少块的信息
2. Wallet: 每个Chain对应一个account，提供账户信息保存、更新功能
3. Market: 包括MarketUniSwapV3, MarketSushiSwap等，根据不同的market就能对应不同的SDK，用于调用对应的Driver
4. Tick: 包括market, coinLeft, coinRight, time(用于表明tick新鲜度), []bid, []ask等
5. TickPool: 提供tick检索获取、写入更新功能
6. Ring: 可获利交易环
7. Setting: 同现有setting设计，用于限制搬砖的范围

# Function models
1. BlockMonitor 
   - 主动轮询或订阅被动获取chain上block信息
   - 调用chain的api计算出tick
   - 将tick放入TickPool成功后，发送给TickChannel由相应的channel handle
2. RingWorker
    - 监听TickChannel,基于获取的Tick尝试build ring
    - 执行ring
3. RingPostHandler
   - 基于执行ring的结果和wallet信息处理上次下单的后续，包括取消、找平。
   - RingPostHandler的触发可能是由chain block打包事件发送到channel触发，也可能根据时间或者某种下单行为触发。总之需要考虑wallet信息的实时同步，可能需要对RingWorker等进行加锁

# TBD
1. 是否通过flash bot进行下单，该决策会影响RingPostHandler的机制设计