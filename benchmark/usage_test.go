package benchmark

import (
	"fmt"
	"net/http"
	"sync"
)

var wg sync.WaitGroup


func sendHTTP(url string) error {
	_, err := http.Get(url)
	defer wg.Done()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}
