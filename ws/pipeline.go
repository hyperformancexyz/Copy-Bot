package ws

import (
	"time"
)

type Distinctable interface {
	DistinctKey() string
}

type Pipeline[T Distinctable] struct {
	out <-chan T
}

func NewPipeline[T Distinctable](in <-chan T) Pipeline[T] {
	return Pipeline[T]{out: in}
}

func (p Pipeline[T]) Out() <-chan T {
	return p.out
}

// ForwardTo spawns a goroutine that reads from this pipeline
// and pushes items into the given external channel, then closes it.
//
// Example usage:
//
//	pipeline.Distinct().DebounceLeading(5 * time.Second).ForwardTo(ch)
func (p Pipeline[T]) ForwardTo(dst chan<- T) Pipeline[T] {
	go func() {
		defer close(dst)
		for v := range p.out {
			dst <- v
		}
	}()
	return p // returning p lets you keep chaining if you want
}

func (p Pipeline[T]) Distinct() Pipeline[T] {
	out := make(chan T)
	go func() {
		defer close(out)
		seen := make(map[string]struct{})
		for val := range p.out {
			k := val.DistinctKey()
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				out <- val
			}
		}
	}()
	return Pipeline[T]{out: out}
}

func (p Pipeline[T]) DebounceLeading(delay time.Duration) Pipeline[T] {
	out := make(chan T)
	go func() {
		defer close(out)
		first := true
		var nextAllowed time.Time
		for val := range p.out {
			now := time.Now()
			if first {
				first = false
				out <- val
				nextAllowed = now.Add(delay)
				continue
			}
			if now.After(nextAllowed) {
				out <- val
				nextAllowed = now.Add(delay)
			}
		}
	}()
	return Pipeline[T]{out: out}
}

func (p Pipeline[T]) Tee(n int) []Pipeline[T] {
	outs := make([]chan T, n)
	pipes := make([]Pipeline[T], n)
	for i := 0; i < n; i++ {
		ch := make(chan T)
		outs[i] = ch
		pipes[i] = Pipeline[T]{out: ch}
	}
	go func() {
		defer func() {
			for _, c := range outs {
				close(c)
			}
		}()
		for val := range p.out {
			for _, c := range outs {
				c <- val
			}
		}
	}()
	return pipes
}
