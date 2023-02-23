package benchmark

import (
	"net/http"
	"testing"
)

/*
goos: windows
goarch: amd64
pkg: github.com/qiancijun/cheryl/benchmark
cpu: Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz
BenchmarkReverse10-8               10000            459640 ns/op
BenchmarkReverse10-12              10000            173317 ns/op
BenchmarkReverse100-8              10000           3057272 ns/op
BenchmarkReverse100-12             10000           2734726 ns/op
BenchmarkReverse1000-8               100          13359845 ns/op
BenchmarkReverse1000-12              100          15632025 ns/op
PASS
ok      github.com/qiancijun/cheryl/benchmark   75.472s
*/

func send(n int) {
	for i := 1; i <= n; i++ {
		go http.Get("http://localhost/api/hello")
	}
}

func BenchmarkReverse10(b *testing.B) {

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		send(10)
	}
}

func BenchmarkReverse100(b *testing.B) {

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		send(100)
	}
}

func BenchmarkReverse1000(b *testing.B) {

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		send(1000)
	}
}