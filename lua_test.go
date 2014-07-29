package lua

import (
	"testing"
	"unsafe"
)

func TestNew(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

func TestSet(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// wrong args
	err = l.Set(nil, nil)
	if err == nil {
		t.Fatal("allowing non-string name")
	}
	err = l.Set("foo")
	if err == nil {
		t.Fatal("allowing wrong number of arguments")
	}

	// namespace
	err = l.Set("foo.bar.baz", func() {})
	if err != nil {
		t.Fatal(err)
	}

	// bad namespace
	l.Set("i", 5)
	err = l.Set("i.foo", 5)
	if err == nil {
		t.Fatalf("allowing bad global namespace")
	}

	// bad namespace
	l.Set("a.b", 5)
	err = l.Set("a.b.c", 5)
	if err == nil {
		t.Fatalf("allowing bad namespace")
	}
}

func TestFunc(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// variadic func
	err = l.Set("foo", func(args ...int) {})
	if err == nil {
		t.Fatalf("allowing variadic func")
	}

	// invoke
	err = l.Set("foo", func() {})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Eval("foo()")
	if err != nil {
		t.Fatal(err)
	}

	// args
	l.Set("bar", func(i int, s string, b bool) {
		if i != 42 {
			t.Fatalf("i is not 42")
		}
		if s != "foobar" {
			t.Fatalf("s is not foobar")
		}
		if b != true {
			t.Fatalf("b is not true")
		}
	})
	l.Eval(`bar(42, 'foobar', true)`)

	// return
	l.Set("baz", func() (int, string, bool) {
		return 42, "foobar", true
	})
	_, err = l.Eval(`
	i, s, b = baz()
	if i ~= 42 then error('i is not 42') end
	if s ~= 'foobar' then error('s is not foobar') end
	if b ~= true then error('b is not true') end
	`)
	if err != nil {
		t.Fatal(err)
	}

	// bad args TODO not testable
	// _, err = l.Eval(`bar(42)`)
}

func TestEval(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// bad code
	_, err = l.Eval("foobar 1, 2, 3")
	if err == nil {
		t.Fatalf("allowing bad code")
	}

	// runtime error
	_, err = l.Eval("error(42)")
	if err == nil {
		t.Fatalf("allowing runtime error")
	}

	// return
	ret, err := l.Eval("return 42")
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 1 {
		t.Fatalf("return value is not 1-sized")
	}
	if i, ok := ret[0].(float64); !ok || i != 42 {
		t.Fatalf("return value is not [42]")
	}

	// return
	ret, err = l.Eval(`return 'foobar', 42, true`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 3 {
		t.Fatalf("return value if not 3-sized")
	}
	if v, ok := ret[0].(string); !ok || v != "foobar" {
		t.Fatalf("return value is not foobar")
	}
	if v, ok := ret[1].(float64); !ok || v != 42 {
		t.Fatalf("return value is not 42")
	}
	if v, ok := ret[2].(bool); !ok || v != true {
		t.Fatalf("return value is not true")
	}

}

func TestSetBool(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Set("T", true)
	ret, err := l.Eval("return T")
	if err != nil || ret[0].(bool) != true {
		t.Fatalf("T is not true")
	}

	l.Set("F", false)
	ret, err = l.Eval("return F")
	if err != nil || ret[0].(bool) != false {
		t.Fatalf("F is not false")
	}
}

func TestSetString(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Set("S", "foobarbaz")
	ret, err := l.Eval("return S")
	if err != nil || ret[0].(string) != "foobarbaz" {
		t.Fatalf("S is not foobarbaz")
	}
}

func TestSetNumber(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Set("I", int64(42))
	ret, err := l.Eval("return I")
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("I is not 42")
	}

	l.Set("U", uint16(42))
	ret, err = l.Eval("return U")
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("U is not 42")
	}

	l.Set("F", float64(42))
	ret, err = l.Eval("return F")
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("F is not 42")
	}
}

func TestSetSlice(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Set("Ints", []int{5, 3, 2, 1, 4})
	_, err = l.Eval(`
	if Ints[1] ~= 5 then error('1 is not 5') end
	if Ints[2] ~= 3 then error('2 is not 3') end
	if Ints[3] ~= 2 then error('3 is not 2') end
	if Ints[4] ~= 1 then error('4 is not 1') end
	if Ints[5] ~= 4 then error('5 is not 4') end
	`)
	if err != nil {
		t.Fatal(err)
	}

	l.Set("Vals", []interface{}{"foobar", 42, true})
	_, err = l.Eval(`
	if Vals[1] ~= 'foobar' then error('1 is not foobar') end
	if Vals[2] ~= 42 then error('2 is not 42') end
	if Vals[3] ~= true then error('3 is not true') end
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetUnsafePointer(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	i := 42
	p := unsafe.Pointer(&i)
	l.Set("P", p)
	ret, err := l.Eval("return P")
	if err != nil {
		t.Fatal(err)
	}
	if *(*int)(ret[0].(unsafe.Pointer)) != 42 {
		t.Fatalf("pointer is not point to 42")
	}
}

func TestSetInterface(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var i interface{}
	i = "foobarbaz"
	l.Set("I", i)
	_, err = l.Eval(`
	if I ~= "foobarbaz" then error('I is not foobarbaz') end
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetPointer(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	i := 42
	l.Set("P", &i)
	ret, err := l.Eval("return P")
	if err != nil || *((*int)(ret[0].(unsafe.Pointer))) != 42 {
		t.Fatal("P is not point to 42")
	}
}
