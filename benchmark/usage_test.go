package benchmark

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

var wg sync.WaitGroup

func TestTime(t *testing.T) {
	total := 10
	begin := time.Now().UnixMilli()
	for i := 1; i <= total; i++ {
		wg.Add(1)
		go sendHTTP("http://localhost/hello")
	}
	wg.Wait()
	end := time.Now().UnixMilli()
	fmt.Println(end - begin)
}

func sendHTTP(url string) error {
	_, err := http.Get(url)
	defer wg.Done()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	return nil
}
