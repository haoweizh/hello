package utils

import "github.com/ethereum/go-ethereum/common"

var Address0 = common.HexToAddress(`0x0000000000000000000000000000000000000000`)

func BigIntIsNegative(bytes []byte) ([]byte, bool) {
	// 如果最高位是 1，说明是负数
	if bytes[0]&0x80 == 0x80 {
		// 将字节切片取反
		for i := range bytes {
			bytes[i] = ^bytes[i]
		}
		// 加 1 得到补码形式
		for i := len(bytes) - 1; i >= 0; i-- {
			bytes[i]++
			// 如果加 1 后不等于 0，说明没有发生溢出，停止加 1 操作
			if bytes[i] != 0 {
				break
			}
		}
		return bytes, true
	}
	return bytes, false
}
