package entity

// Start
// 1. load from db to get current status
// 2. loop read block
// 3. throw into channel
func Start() {
	msg := ``
	ChanBlock <- msg
}
