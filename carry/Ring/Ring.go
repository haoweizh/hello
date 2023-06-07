package Ring

import (
	"hello/model"
	"sync"
)

var Pairs = sync.Map{}
var Pools1 = make([]*model.Pool, 0)

func InitRingPool() {

}

// EnhanceRingPool 假定当前是n阶，增强为n+1阶
func EnhanceRingPool(stage int) (enhanced bool) {
	Pairs.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		pair := value.(*model.Pair)
		if len(pair.Pools) < stage {
			return true
		}
		pairAdd := &model.Pair{}
		for _, pool := range Pools1 {
			if pair.GetTail() != pool.Token0 {
				continue
			}
			pairAdd.Pools = append(pair.Pools, pool)
			//TODO 计算pair的价格pairAdd.Price = pair.Price + pool.DealPrice(pool)
			if AddPair(pairAdd) {
				enhanced = true
			}
		}
		return true
	})
	if enhanced {
		enhanced = EnhanceRingPool(stage + 1)
	}
	return enhanced
}

func AddPair(addPair *model.Pair) (success bool) {
	Pairs.Range(func(key, value any) bool {
		if value == nil {
			return true
		}
		pair := value.(*model.Pair)
		if pair.GetKey() == addPair.GetKey() && pair.Price < addPair.Price {
			Pairs.Store(addPair.GetKey(), addPair)
			success = true
			return false
		}
		return true
	})
	value, _ := Pairs.Load(addPair.GetKey())
	if !success && value == nil {
		Pairs.Store(addPair.GetKey(), addPair)
		success = true
	}
	return success
}
