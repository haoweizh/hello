package Ring

import (
	"hello/model"
)

var CompleteRings = make(map[string]*model.Ring)

func InitRingPool() {

}

// EnhanceRingPool 假定当前是n阶，增强为n+1阶
func addStep(stepRings map[string]*model.Ring, settings []*model.Setting) (nextRings map[string]*model.Ring) {
	nextRings = make(map[string]*model.Ring)
	for _, ring := range stepRings {
		for _, setting := range settings {
			copyRing := ring.Copy()
			if copyRing.AddSetting(setting) {
				if copyRing.Complete {
					CompleteRings[copyRing.Key] = copyRing
				} else {
					nextRings[copyRing.Key] = copyRing
				}
			}
		}
	}
	if len(nextRings) > 0 {
		addStep(nextRings, settings)
	}
	return nextRings
}
