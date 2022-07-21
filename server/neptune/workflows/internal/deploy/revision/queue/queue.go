package queue

type Message struct {
	Revision string
}

// Queue is a standard queue implementation
// TODO: fill me in
type Queue struct{}

func (q *Queue) IsEmpty() bool {
	return false
}

func (q *Queue) Push(msg Message) {

}

func (q *Queue) Pop() Message {
	return Message{}
}
