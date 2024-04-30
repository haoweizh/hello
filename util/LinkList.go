package util

type Node struct {
	Next, Prev *Node
	Data       interface{}
}

type LinkList struct {
	Head, Tail *Node
	Len        int
}

// Insert add nodeAdd between nodeCurrent and nodeNext
func (linkList *LinkList) Insert(nodeCurrent, nodeAdd *Node) {
	if nodeAdd == nil || nodeCurrent == nil {
		return
	}
	if nodeCurrent.Next == nil {
		linkList.AddTailData(nodeAdd)
		return
	}
	nodeAdd.Prev = nodeCurrent
	nodeAdd.Next = nodeCurrent.Next
	nodeCurrent.Next = nodeAdd
	if nodeAdd.Next != nil {
		nodeAdd.Next.Prev = nodeAdd
	}
	linkList.Len++
}

func (linkList *LinkList) AddHeadData(data interface{}) {
	node := &Node{Data: data}
	if linkList.Head == nil || linkList.Tail == nil {
		linkList.Head = node
		linkList.Tail = node
	} else {
		node.Next = linkList.Head
		linkList.Head.Prev = node
		linkList.Head = node
	}
	linkList.Len++
}

func (linkList *LinkList) AddTailData(data interface{}) bool {
	node := &Node{Data: data}
	if linkList.Head == nil || linkList.Tail == nil {
		linkList.Head = node
		linkList.Tail = node
	} else {
		node.Prev = linkList.Tail
		linkList.Tail.Next = node
		linkList.Tail = node
	}
	linkList.Len++
	return true
}

func (linkList *LinkList) RemoveHead() bool {
	if linkList.Head == nil || linkList.Tail == nil {
		return false
	}
	headOld := linkList.Head
	headNew := linkList.Head.Next
	headOld.Next = nil
	if headNew != nil {
		headNew.Prev = nil
	} else {
		linkList.Tail = nil
	}
	linkList.Head = headNew
	linkList.Len--
	return true
}
