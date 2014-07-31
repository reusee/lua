package lua

import (
	"fmt"
	"strings"
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
	err = l.Pset(nil, nil)
	if err == nil {
		t.Fatal("allowing non-string name")
	}
	err = l.Pset("foo")
	if err == nil {
		t.Fatal("allowing wrong number of arguments")
	}

	// namespace
	err = l.Pset("foo.bar.baz", func() {})
	if err != nil {
		t.Fatal(err)
	}

	// bad namespace
	l.Pset("i", 5)
	err = l.Pset("i.foo", 5)
	if err == nil {
		t.Fatalf("allowing bad global namespace")
	}

	// bad namespace
	l.Pset("a.b", 5)
	err = l.Pset("a.b.c", 5)
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
	err = l.Pset("foo", func(args ...int) {})
	if err == nil {
		t.Fatalf("allowing variadic func")
	}

	// invoke
	err = l.Pset("foo", func() {})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Peval("foo()")
	if err != nil {
		t.Fatal(err)
	}

	// args
	l.Pset("bar", func(i int, s string, b bool) {
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
	l.Peval(`bar(42, 'foobar', true)`)

	// return
	l.Pset("baz", func() (int, string, bool) {
		return 42, "foobar", true
	})
	_, err = l.Peval(`
	i, s, b = baz()
	if i ~= 42 then error('i is not 42') end
	if s ~= 'foobar' then error('s is not foobar') end
	if b ~= true then error('b is not true') end
	`)
	if err != nil {
		t.Fatal(err)
	}

	// bad args
	_, err = l.Peval(`
	(function()
		(function()
			bar(42)
		end)()
	end)()
	`)
	if err == nil {
		t.Fatalf("allowing bad args")
	}
	msg := fmt.Sprintf("%s", err)
	if !strings.Contains(msg, "number of arguments not match") || !strings.Contains(msg, "stack traceback:") {
		fmt.Printf("%s\n", msg)
		t.Fatalf("incorrect error message")
	}

	// stack trace
	l.Peval(`bar(42)`)
	l.Peval(`bar(42)`)
	l.Peval(`bar(42)`)
	l.Peval(`bar(42)`)
	_, err = l.Peval(`
		(function()
			(function()
				error('Error, Error, Error.')
			end)()
		end)()
		`)
	if err == nil {
		t.Fatalf("allowing error()")
	}
	msg = fmt.Sprintf("%s", err)
	if !strings.Contains(msg, "Error, Error, Error.") || !strings.Contains(msg, "stack traceback:") {
		fmt.Printf("%s\n", msg)
		t.Fatalf("incorrect error message")
	}
}

func TestEval(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// bad code
	_, err = l.Peval("foobar 1, 2, 3")
	if err == nil {
		t.Fatalf("allowing bad code")
	}

	// runtime error
	_, err = l.Peval("error(42)")
	if err == nil {
		t.Fatalf("allowing runtime error")
	}

	// return
	ret, err := l.Peval("return 42")
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
	ret, err = l.Peval(`return 'foobar', 42, true`)
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

	l.Pset("T", true)
	ret, err := l.Peval("return T")
	if err != nil || ret[0].(bool) != true {
		t.Fatalf("T is not true")
	}

	l.Pset("F", false)
	ret, err = l.Peval("return F")
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

	l.Pset("S", "foobarbaz")
	ret, err := l.Peval("return S")
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

	l.Pset("I", int64(42))
	ret, err := l.Peval("return I")
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("I is not 42")
	}

	l.Pset("U", uint16(42))
	ret, err = l.Peval("return U")
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("U is not 42")
	}

	l.Pset("F", float64(42))
	ret, err = l.Peval("return F")
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

	l.Pset("Ints", []int{5, 3, 2, 1, 4})
	_, err = l.Peval(`
	if Ints[1] ~= 5 then error('1 is not 5') end
	if Ints[2] ~= 3 then error('2 is not 3') end
	if Ints[3] ~= 2 then error('3 is not 2') end
	if Ints[4] ~= 1 then error('4 is not 1') end
	if Ints[5] ~= 4 then error('5 is not 4') end
	`)
	if err != nil {
		t.Fatal(err)
	}

	l.Pset("Vals", []interface{}{"foobar", 42, true})
	_, err = l.Peval(`
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
	l.Pset("P", p)
	ret, err := l.Peval("return P")
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
	l.Pset("I", i)
	_, err = l.Peval(`
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
	l.Pset("P", &i)
	ret, err := l.Peval("return P")
	if err != nil || *((*int)(ret[0].(unsafe.Pointer))) != 42 {
		t.Fatal("P is not point to 42")
	}
}

func TestCall(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// call
	l.Set("foo", func(i int, s string, b bool) (bool, int, string) {
		return b, i, s
	})
	ret, err := l.Pcall("foo", 42, "foobar", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 3 {
		t.Fatalf("number of returns not match")
	}
	if ret[0].(bool) != true {
		t.Fatalf("0 is not true")
	}
	if ret[1].(float64) != 42 {
		t.Fatalf("1 is not 42")
	}
	if ret[2].(string) != "foobar" {
		t.Fatalf("2 is not foobar")
	}

	// bad argument number
	_, err = l.Pcall("foo", 42)
	if err == nil {
		t.Fatalf("allowing bad call")
	}

	// bad argument type
	_, err = l.Pcall("foo", true, 42, "justwe")
	if err == nil {
		t.Fatalf("allowing bad argument type")
	}

	// bad function
	_, err = l.Pcall("bar")
	if err == nil {
		t.Fatalf("allowing bad function")
	}

	// bad function
	_, err = l.Pcall("foo.bar.baz")
	if err == nil {
		t.Fatalf("allowing bad function path")
	}

	// call
	l.Eval(`
	baz = {
		bar = {
			foo = function()
				error('foo error')
			end,
			bar = function(n)
				return n * 2
			end,
		}
	}
	`)
	_, err = l.Pcall("baz.bar.foo")
	if err == nil || !strings.Contains(err.Error(), "foo error") {
		t.Fatalf("allowing error")
	}

	// call
	if l.Call("baz.bar.bar", 42)[0].(float64) != 84 {
		t.Fatalf("return is not 84")
	}

	// call
	l.Eval(`
	function get()
		return 42
	end
	function set(n)
		return n * 2
	end
	`)
	ret, err = l.Pcall("set", l.Call("get")[0].(float64))
	if err != nil || ret[0].(float64) != 84 {
		t.Fatalf("result error")
	}
}

func TestEvalEnvs(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// bind
	ret, err := l.Peval(`return V`, "V", 42)
	if err != nil || ret[0].(float64) != 42 {
		t.Fatalf("V is not 42 or error %v", err)
	}

	// global vars
	l.Set("foo", 42)
	ret, err = l.Peval(`return foo + bar`, "bar", 42)
	if err != nil || ret[0].(float64) != 84 {
		t.Fatalf("return is not 84 or error %v", err)
	}
	// env scope
	ret, err = l.Peval(`return bar`)
	if err != nil || ret[0] != nil {
		t.Fatalf("bar is leak to global or error %v", err)
	}
}
