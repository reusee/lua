package lua

import "testing"

func BenchmarkSet(b *testing.B) {
	l, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer l.Close()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l.Set("foo.bar.baz", "bar")
	}
}
