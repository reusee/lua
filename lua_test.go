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

	// bad envs arg
	_, err = l.Peval(`return 42`, "foo")
	if err == nil || !strings.Contains(err.Error(), "number of arguments not match") {
		t.Fatalf("allowing bad envs args")
	}
	_, err = l.Peval(`return true`, 42, 42)
	if err == nil || !strings.Contains(err.Error(), "name must be string") {
		t.Fatalf("allowing bad envs args")
	}
}

func TestPanic(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatalf("Set no panic")
			}
		}()
		l.Set("foo") // bad argc
	}()

	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatalf("Eval no panic")
			}
		}()
		l.Eval(`foo()`)
	}()

	func() {
		defer func() {
			if e := recover(); e == nil {
				t.Fatalf("Call no panic")
			}
		}()
		l.Call("none")
	}()
}

func TestTypeConvert(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	// go value to lua value
	l.Eval(`
	function foo(...)
		for i, v in ipairs({...}) do
			if v ~= 42 then error('not 42') end
		end
	end
	`)
	i := 42
	var ii interface{}
	ii = i
	_, err = l.Pcall("foo",
		int8(42),
		int16(42),
		int32(42),
		int64(42),
		uint(42),
		uint8(42),
		uint16(42),
		uint32(42),
		uint64(42),
		float32(42),
		float64(42),
		ii,
	)
	if err != nil {
		t.Fatal(err)
	}

	// nil value
	l.Set("bar", func(i interface{}) {
		if i != nil {
			t.Fatalf("not nil")
		}
	})
	_, err = l.Peval(`bar(nil)`)
	if err != nil {
		t.Fatal(err)
	}

	// lua value to go value
	l.Set("baz", func(i1 uint, i2 uint8, i3 uint16, i4 uint32, i5 uint64,
		i6 float32, i7 float64) {
		if i1 != 42 {
			t.Fatalf("wrong arg")
		}
		if i2 != 42 {
			t.Fatalf("wrong arg")
		}
		if i3 != 42 {
			t.Fatalf("wrong arg")
		}
		if i4 != 42 {
			t.Fatalf("wrong arg")
		}
		if i5 != 42 {
			t.Fatalf("wrong arg")
		}
		if i6 != 42 {
			t.Fatalf("wrong arg")
		}
		if i7 != 42 {
			t.Fatalf("wrong arg")
		}
	})
	_, err = l.Peval(`baz(42, 42, 42, 42, 42, 42, 42)`)
	if err != nil {
		t.Fatal(err)
	}

	// byte slice
	l.Set("baz", func(bs []byte) {
		if string(bs) != "foobarbaz" {
			t.Fatalf("bytes is not foobarbaz")
		}
	})
	_, err = l.Peval(`baz('foobarbaz')`)
	if err != nil {
		t.Fatal(err)
	}

	// pointer
	i = 42
	l.Set("ip", &i)
	l.Set("baz", func(ip *int) {
		if *ip != 42 {
			t.Fatalf("not point to 42")
		}
	})
	l.Eval(`baz(ip)`)
	l.Set("baz", func(ip unsafe.Pointer) {
		if *(*int)(ip) != 42 {
			t.Fatalf("not point to 42")
		}
	})
	l.Eval(`baz(ip)`)

	// map
	l.Set("baz", func(m map[string]int) {
		if len(m) != 3 {
			t.Fatalf("map is not 3 sized")
		}
		if m["foo"] != 42 || m["bar"] != 42 || m["baz"] != 42 {
			t.Fatalf("map value error")
		}
	})
	l.Eval(`baz{
		foo = 42,
		bar = 42,
		baz = 42,
	}`)

	// wrong type
	l.Set("baz", func(b bool) {})
	_, err = l.Pcall("baz", 42)
	if err == nil || !strings.Contains(err.Error(), "not a boolean") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg uint) {})
	_, err = l.Pcall("baz", true)
	if err == nil || !strings.Contains(err.Error(), "not a unsigned") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg float64) {})
	_, err = l.Pcall("baz", true)
	if err == nil || !strings.Contains(err.Error(), "not a float") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg string) {})
	_, err = l.Pcall("baz", 42)
	if err == nil || !strings.Contains(err.Error(), "not a string") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg []int) {})
	_, err = l.Pcall("baz", 42)
	if err == nil || !strings.Contains(err.Error(), "wrong slice argument") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg *int) {})
	_, err = l.Pcall("baz", 42)
	if err == nil || !strings.Contains(err.Error(), "not a pointer") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(arg map[int]int) {})
	_, err = l.Pcall("baz", 42)
	if err == nil || !strings.Contains(err.Error(), "not a map") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}

	// unsupported interface
	l.Set("baz", func(arg error) {})
	_, err = l.Pcall("baz", fmt.Errorf("error"))
	if err == nil || !strings.Contains(err.Error(), "only interface{} is supported, no error") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set("baz", func(i interface{}) {})
	_, err = l.Peval(`baz(function()end)`)
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}

	// unsupported type
	err = l.Pset("foo", struct{}{})
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("allowing unsupported type or error %v", err)
	}
	err = l.Pset("foo", []struct{}{struct{}{}})
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Set(`foo`, func(arg []interface{}) {})
	_, err = l.Pcall("foo", []interface{}{func() {}})
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	_, err = l.Peval("foo(T)", "T", []interface{}{func() {}})
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	_, err = l.Peval(`return (function() end)`)
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	_, err = l.Peval(`return T`, "T", struct{}{})
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("allowing unsupported type or error %v", err)
	}
	l.Set("foo", func(arg func()) {})
	_, err = l.Pcall("foo", func() {})
	if err == nil || !strings.Contains(err.Error(), "unsupported toGoValue type func()") {
		t.Fatalf("allowing unsupported type or error %v", err)
	}
	l.Set("foo", func(arg map[string]func()) {})
	_, err = l.Peval(`foo{k = function()end}`)
	if err == nil || !strings.Contains(err.Error(), "unsupported toGoValue type func()") {
		t.Fatalf("allowing unsupported type or error %v", err)
	}
	l.Set("foo", func(arg map[interface{}]interface{}) {})
	_, err = l.Peval(`foo{[function()end] = 5}`)
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}
	l.Eval(`function foo() return (function() end) end`)
	_, err = l.Pcall("foo")
	if err == nil || !strings.Contains(err.Error(), "unsupported type FUNCTION for interface{}") {
		t.Fatalf("allowing wrong type of arg or error %v", err)
	}

	// int slice
	l.Set("baz", func(s []int) {
		if len(s) != 3 {
			t.Fatalf("slice is not 3 sized")
		}
		for _, i := range s {
			if i != 42 {
				t.Fatalf("slice elem is not 42")
			}
		}
	})
	l.Eval(`baz{42, 42, 42}`)

	// nil
	l.Set("baz", func(i interface{}) {
		if i != nil {
			t.Fatalf("i is not nil")
		}
	})
	l.Call("baz", nil)

	// type name
	coverLuaTypeName()
}

func TestUnicode(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ret := l.Eval(`return T`, "T", "你好")
	if len(ret) != 1 || ret[0] != "你好" {
		t.Fatal()
	}
}
