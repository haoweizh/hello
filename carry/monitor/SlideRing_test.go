package monitor

import (
	"fmt"
	"testing"
)

func Test_ClearSpot(t *testing.T) {
	sr := SlideRing{
		start:   0,
		current: 0,
		data:    nil,
	}
	for i := 1; i < 10; i++ {
		sr.add(i)
	}
	for i := 1; i < 5; i++ {
		sr.remove()
	}
	for i := 1; i < 15; i++ {
		fmt.Println(sr.remove())
	}
	fmt.Println(sr.get())
}
