package carry

// get usd balance
// setting with open future amount, present amount
// open amount: min(usd value - all current other coin value, best bid amount, best ask amount)
// open price: lose 0.0014 price distance < last 6 hour funding rate * 4, ioc
// close price: win 0.0014 price distance or last 6 hour funding rate * 4, market
