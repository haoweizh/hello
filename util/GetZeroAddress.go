package util

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//	concurrency = flag.Int("c", 10, "concurrency")
	//	number      = flag.Int64("n", 1, "number")
	//	pattern     = flag.String("p", "000000000000", "pattern")
	contractNum = sync.Map{}
	mining      = false
)

//func init() {
//	flag.Parse()
//}

func RunMindZeroAddr(zeroLenFrom, zerosLenTo, concurrency int) (msg string) {
	got := false
	if mining {
		contractNum.Range(func(key, value any) bool {
			if value != nil {
				msg += fmt.Sprintf("%v0 contracts got %d\n", key, len(value.([]string)))
				for _, s := range value.([]string) {
					msg += fmt.Sprintf("%s\n", s)
				}
				got = true
			}
			return true
		})
		if !got {
			msg = `mining, not yet found`
		} else {
			msg = "mining, got\n" + msg
		}
	} else {
		go MindZeroAddr(zeroLenFrom, zerosLenTo, concurrency)
		msg = `mining started`
	}
	return
}

func MindZeroAddr(zeroLenFrom, zerosLenTo, concurrency int) {
	if zerosLenTo < zeroLenFrom {
		return
	}
	defer func() { mining = false }()
	mining = true
	patterns := make([]*regexp.Regexp, zerosLenTo-zeroLenFrom+1)
	for i := 0; i < len(patterns); i++ {
		patterns[i] = regexp.MustCompile(fmt.Sprintf(`^0x0{%d}`, i+zeroLenFrom))
	}
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 1; i <= concurrency; i++ {
		go func() {
			defer wg.Done()
			getZeroAddr(patterns, uint64(0))
		}()
	}
	wg.Wait()
}

func getZeroAddr(patterns []*regexp.Regexp, nonce uint64) {
	number := int64(1)
	for number > 0 {
		key, _ := crypto.GenerateKey()
		address := crypto.PubkeyToAddress(key.PublicKey)
		contract := crypto.CreateAddress(address, nonce).Hex()
		for i := 0; i < len(patterns); i++ {
			if !patterns[i].MatchString(contract) {
				continue
			}
			value, _ := contractNum.Load(patterns[i].String())
			if value != nil {
				contracts := append(value.([]string), fmt.Sprintf(`%s Get %s`, time.Now().String(), contract))
				contractNum.Store(patterns[i].String(), contracts)
			} else {
				contractNum.Store(patterns[i].String(), []string{fmt.Sprintf(`%s Get %s`, time.Now().String(), contract)})
			}
			if i == len(patterns)-1 {
				if atomic.AddInt64(&number, -1) < 0 {
					break
				}
			}
			privateKey := hex.EncodeToString(key.D.Bytes())
			Log(LogLevelInfo, fmt.Sprintf("Contract:\t%s\nAddress:\t%s\nPrivateKey:\t%s\n\n",
				contract, address.Hex(), privateKey))
		}
	}
}
