package util

import (
	"encoding/hex"
	"flag"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/crypto"
)

var (
	concurrency = flag.Int("c", 10, "concurrency")
	number      = flag.Int64("n", 1, "number")
	pattern     = flag.String("p", "000000000000", "pattern")
)

func init() {
	flag.Parse()
}

func main() {
	var wg sync.WaitGroup

	wg.Add(*concurrency)

	reg := regexp.MustCompile("^0x" + *pattern)
	nonce := uint64(0)

	for i := 0; i < *concurrency; i++ {
		go func() {
			defer wg.Done()
			run(reg, nonce)
		}()
	}

	wg.Wait()
}

func run(reg *regexp.Regexp, nonce uint64) {
	for *number > 0 {
		key, _ := crypto.GenerateKey()
		address := crypto.PubkeyToAddress(key.PublicKey)
		contract := crypto.CreateAddress(address, nonce).Hex()

		if !reg.MatchString(contract) {
			continue
		}

		if atomic.AddInt64(number, -1) < 0 {
			break
		}

		privateKey := hex.EncodeToString(key.D.Bytes())

		fmt.Printf("Contract:\t%s\nAddress:\t%s\nPrivateKey:\t%s\n\n",
			contract,
			address.Hex(),
			privateKey,
		)
	}
}
