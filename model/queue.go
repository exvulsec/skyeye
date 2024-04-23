package model

type Type interface {
	*int64 | *TransactionTrace
}

type Queue[T Type] struct {
	items []T
}

func (q *Queue[T]) Push(item T) {
	q.items = append(q.items, item)
}

func (q *Queue[T]) Pop() T {
	if !q.IsEmpty() {
		item := q.items[0]
		q.items = q.items[1:]
		return item
	}
	return nil
}

func (q *Queue[T]) IsEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue[T]) Top() T {
	if !q.IsEmpty() {
		return q.items[0]
	}
	return nil
}
