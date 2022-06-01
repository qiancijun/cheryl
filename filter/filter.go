package filter

import (
	"net/http"
)

type FilterFunc func(w http.ResponseWriter, r *http.Request) error

type Filter struct {
	fun  FilterFunc
	next *Filter
}

var (
	FilterChain *Filter
)

func NewFilter(fun FilterFunc) *Filter {
	return &Filter{
		fun: fun,
		next: nil,
	}
}

func CreateFilterChain(filters... *Filter) {
	if len(filters) == 0 {
		return
	}
	if len(filters) == 1 {
		FilterChain = filters[0]
	}
	head, tail := filters[0], filters[0]
	for i := 1; i < len(filters); i++ {
		cur := filters[i]
		tail.next = cur
		tail = tail.next
	}
	FilterChain = head
}

func ExecuteFilterChain(w http.ResponseWriter, r *http.Request) error {
	if FilterChain == nil {
		return nil
	}
	var err error
	root := FilterChain
	for root != nil {
		err = root.fun(w, r)
		if err != nil {
			return err
		}
		root = root.next
	}
	return nil
}