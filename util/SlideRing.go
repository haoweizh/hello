package util

import (
	"fmt"
)

type SlideRing struct {
	Start, Current int // Start == current代表空
	Data           []interface{}
}

func (slideRing *SlideRing) Add(value interface{}) {
	if slideRing.Data == nil {
		slideRing.Data = make([]interface{}, 128)
	}
	slideRing.Current = (slideRing.Current + 1) % len(slideRing.Data)
	if slideRing.Start == slideRing.Current {
		data := make([]interface{}, len(slideRing.Data)*2)
		for i := slideRing.Start; i < len(slideRing.Data); i++ {
			data[i-slideRing.Start] = slideRing.Data[i]
		}
		for i := 0; i < slideRing.Start; i++ {
			data[i+len(slideRing.Data)-slideRing.Start] = slideRing.Data[i]
		}
		slideRing.Start = 0
		slideRing.Current = len(slideRing.Data)
		slideRing.Data = data
	}
	slideRing.Data[slideRing.Current] = value
}

func (slideRing *SlideRing) GetIndex(index int) interface{} {
	if slideRing.Data == nil || len(slideRing.Data) < index {
		return nil
	}
	return slideRing.Data[index]
}

func (slideRing *SlideRing) Get() (start, current interface{}) {
	if slideRing.Data != nil {
		if slideRing.Start < len(slideRing.Data) {
			start = slideRing.Data[slideRing.Start]
		}
		if slideRing.Current < len(slideRing.Data) {
			current = slideRing.Data[slideRing.Current]
		}
	}
	return
}

func (slideRing *SlideRing) Remove() (success bool) {
	if slideRing.Start == slideRing.Current {
		Log(``, LogLevelInfo, ``, SystemCarry, fmt.Sprintf(`sliding ring Start reach Current %d`, slideRing.Start))
		return false
	}
	slideRing.Start = (slideRing.Start + 1) % len(slideRing.Data)
	return true
}
