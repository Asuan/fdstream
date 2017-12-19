package fdstream

import (
	"testing"
)

type a struct {
	gi int
}

type b struct {
	a
}

type c struct {
	a *a
}

type d struct {
	*a
}

type e struct {
	b
}

func (a *a) ting(i int) int {
	return i + a.gi
}

func (a *c) ting(i int) int {
	return a.a.ting(i)
}

type Tinger interface {
	ting(int) int
}

func Benchmark_Nesting(br *testing.B) {
	br.StopTimer()

	benchmarks := []struct {
		name string

		tinger Tinger
	}{
		{"root-5", new(a)},
		{"child-5", new(b)},
		{"as-value-5", &c{a: new(a)}},
		{"as ref child-5", &d{a: new(a)}},
		{"child-2lvl-5", new(e)},
	}
	br.StartTimer()
	for _, bm := range benchmarks {
		br.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.tinger.ting(2)
			}
		})
	}
}
