package utils

import (
	"github.com/ethereum/go-ethereum/common"
	"math"
	"math/big"
)

var Address0 = common.HexToAddress(`0x0000000000000000000000000000000000000000`)
var (
	U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF, _ = big.NewInt(0).SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)
	U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF, _                 = big.NewInt(0).SetString("115792089237316195423570985008687907853269984665640564039457584007913129639936", 10)
	U256_0X100000000, _                                        = big.NewInt(0).SetString("4294967296", 10)
	U256_0X10000, _                                            = big.NewInt(0).SetString("65536", 10)
	U256_0X100, _                                              = big.NewInt(0).SetString("256", 10)
	U256_255, _                                                = big.NewInt(0).SetString("255", 10)
	U256_192, _                                                = big.NewInt(0).SetString("192", 10)
	U256_191, _                                                = big.NewInt(0).SetString("191", 10)
	U256_128, _                                                = big.NewInt(0).SetString("128", 10)
	U256_64, _                                                 = big.NewInt(0).SetString("64", 10)
	U256_32, _                                                 = big.NewInt(0).SetString("32", 10)
	U256_16, _                                                 = big.NewInt(0).SetString("16", 10)
	U256_8, _                                                  = big.NewInt(0).SetString("8", 10)
	U256_4, _                                                  = big.NewInt(0).SetString("4", 10)
	U256_2, _                                                  = big.NewInt(0).SetString("2", 10)
)

type ArithmeticError struct {
	msg string
}

func (e *ArithmeticError) Error() string {
	return e.msg
}

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

// 将 Q64 固定点数转换为 Q16 固定点数
func Q64ToQ16(x uint64) uint64 {
	decimals := (x & 0xFFFFFFFFFFFFFFFF) >> 48
	integers := (x >> 64) & 0xFFFF

	return (integers << 16) + decimals
}

// 将 Q64 固定点数转换为 float64 浮点数
func q64ToF64(x uint64) float64 {
	q16Value := Q64ToQ16(x)
	return q16ToF64(q16Value)
}

// 将 Q16 固定点数转换为 float64 浮点数
func q16ToF64(x uint64) float64 {
	return float64(x) / math.Pow(2, 16)
}
func Div64X64(x, y *big.Int) (*big.Int, error) {
	if y.Sign() != 0 {
		var answer *big.Int

		if x.Cmp(U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF) <= 0 {
			answer = new(big.Int).Mul(x, new(big.Int).Lsh(big.NewInt(1), 64))
			answer = answer.Div(answer, y)
		} else {
			msb := U256_192
			xc := new(big.Int).Rsh(x, 192)

			if xc.Cmp(U256_0X100000000) >= 0 {
				xc = new(big.Int).Rsh(xc, 32)
				msb = msb.Add(msb, U256_32)
			}

			if xc.Cmp(U256_0X10000) >= 0 {
				xc = new(big.Int).Rsh(xc, 16)
				msb = msb.Add(msb, U256_16)
			}

			if xc.Cmp(U256_0X100) >= 0 {
				xc = new(big.Int).Rsh(xc, 8)
				msb = msb.Add(msb, U256_8)
			}

			if xc.Cmp(U256_16) >= 0 {
				xc = new(big.Int).Rsh(xc, 4)
				msb = msb.Add(msb, U256_4)
			}

			if xc.Cmp(U256_4) >= 0 {
				xc = new(big.Int).Rsh(xc, 2)
				msb = msb.Add(msb, U256_2)
			}

			if xc.Cmp(U256_2) >= 0 {
				msb = msb.Add(msb, big.NewInt(1))
			}

			exp := new(big.Int).Sub(U256_255, msb)
			exp = new(big.Int).Lsh(exp, 192)

			num := new(big.Int).Lsh(big.NewInt(1), 191)
			num = new(big.Int).Sub(num, new(big.Int).SetUint64(1))

			tmep := msb.Sub(msb, U256_191).Uint64()

			den := new(big.Int).Rsh(y.Sub(y, big.NewInt(1)), uint(tmep))

			den = new(big.Int).Add(den, big.NewInt(1))

			answer = new(big.Int).Mul(x, exp)
			answer = answer.Div(answer, new(big.Int).Add(den, num))
		}

		// 判断是否溢出
		overflow := new(big.Int).Sub(answer, U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
		if overflow.Cmp(big.NewInt(0)) > 0 {
			return nil, &ArithmeticError{msg: "ShadowOverflow"}
		}

		hi := new(big.Int).Mul(answer, new(big.Int).Rsh(y, 128))
		lo := new(big.Int).Mul(answer, new(big.Int).And(y, U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF))

		xh := new(big.Int).Rsh(x, 192)
		xl := new(big.Int).Lsh(x, 64)

		if xl.Cmp(lo) < 0 {
			xh = xh.Sub(xh, big.NewInt(1))
		}

		xl = xl.Sub(xl, lo)
		lo = lo.Lsh(hi, 128)

		if xl.Cmp(lo) < 0 {
			xh = xh.Sub(xh, big.NewInt(1))
		}

		xl = xl.Sub(xl, lo)

		if xh.Cmp(new(big.Int).Rsh(hi, 128)) != 0 {
			return nil, &ArithmeticError{msg: "RoundingError"}
		}

		answer = answer.Add(answer, new(big.Int).Div(xl, y))
		// 判断是否溢出
		overflow = new(big.Int).Sub(answer, U256_0XFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
		if overflow.Cmp(big.NewInt(0)) > 0 {
			return nil, &ArithmeticError{msg: "ShadowOverflow"}
		}

		return answer, nil
	}

	return nil, &ArithmeticError{msg: "YIsZero"}
}
