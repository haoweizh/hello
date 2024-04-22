package monitor

import (
	"fmt"
	"hello/util"
)

type SlideRing struct {
	start, current int // start == current代表空
	data           []interface{}
}

func (slideRing *SlideRing) add(value interface{}) {
	if slideRing.data == nil {
		slideRing.data = make([]interface{}, 128)
	}
	slideRing.current = (slideRing.current + 1) % len(slideRing.data)
	if slideRing.start == slideRing.current {
		data := make([]interface{}, len(slideRing.data)*2)
		for i := slideRing.start; i < len(slideRing.data); i++ {
			data[i-slideRing.start] = slideRing.data[i]
		}
		for i := 0; i < slideRing.start; i++ {
			data[i+len(slideRing.data)-slideRing.start] = slideRing.data[i]
		}
		slideRing.start = 0
		slideRing.current = len(slideRing.data)
		slideRing.data = data
	}
	slideRing.data[slideRing.current] = value
}

func (slideRing *SlideRing) get() (start, current interface{}) {
	if slideRing.data != nil {
		if slideRing.start < len(slideRing.data) {
			start = slideRing.data[slideRing.start]
		}
		if slideRing.current < len(slideRing.data) {
			current = slideRing.data[slideRing.current]
		}
	}
	return
}

func (slideRing *SlideRing) remove() (success bool) {
	if slideRing.start == slideRing.current {
		util.Info(fmt.Sprintf(`sliding ring start reach current %d`, slideRing.start))
		return false
	}
	slideRing.start = (slideRing.start + 1) % len(slideRing.data)
	return true
}
