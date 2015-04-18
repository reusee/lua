package lua

/*
#include <lua.h>
#include <lualib.h>
#include <lauxlib.h>
#include <string.h>
#include <stdlib.h>
#include <stdint.h>

void push_go_func(lua_State*, int64_t);
void push_errfunc(lua_State*);

lua_State* new_state();
char* ensure_name(lua_State*, char*);
void set_eval_env(lua_State*);

#cgo pkg-config: luajit
*/
import "C"
import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

// Lua struct wraps lua vm state
type Lua struct {
	State *C.lua_State
	err   error
}

type _Function struct {
	name      string
	lua       *Lua
	fun       interface{}
	funcType  reflect.Type
	funcValue reflect.Value
	argc      int
}

var newState = func() *C.lua_State {
	return C.new_state()
}

var dumbNewState = func() *C.lua_State {
	return nil
}

var (
	funcs     []*_Function
	funcsLock sync.RWMutex
)

// New creates a new lua vm
func New() (*Lua, error) {
	state := newState()
	if state == nil {
		return nil, fmt.Errorf("lua newstate")
	}
	lua := &Lua{
		State: state,
	}
	return lua, nil
}

// Pset sets lua variable. no panic when error occur.
func (l *Lua) Pset(args ...interface{}) error {
	if len(args)%2 != 0 {
		return fmt.Errorf("number of arguments not match")
	}
	for i := 0; i < len(args); i += 2 {
		name, ok := args[i].(string)
		if !ok {
			return fmt.Errorf("name must be string, not %v", args[i])
		}
		err := l.set(name, args[i+1])
		if err != nil {
			return err
		}
	}
	return nil
}

// Set sets lua variable. panic if error occur.
func (l *Lua) Set(args ...interface{}) {
	err := l.Pset(args...)
	if err != nil {
		panic(err)
	}
}

func (l *Lua) set(fullname string, v interface{}) error {
	// ensure name
	errMsg := C.ensure_name(l.State, C.CString(fullname))
	if errMsg != nil {
		return fmt.Errorf("%s: %s", errMsg, fullname)
	}

	// push value
	err := l.pushGoValue(v, fullname)
	if err != nil {
		return err
	}

	// set
	C.lua_rawset(l.State, -3)
	// clear stack
	C.lua_settop(l.State, 0)

	return nil
}

func (l *Lua) pushGoValue(v interface{}, name string) error {
	if v == nil {
		C.lua_pushnil(l.State)
		return nil
	}
	switch value := v.(type) {
	case bool:
		if value {
			C.lua_pushboolean(l.State, C.int(1))
		} else {
			C.lua_pushboolean(l.State, C.int(0))
		}
	case string:
		C.lua_pushstring(l.State, C.CString(value))
	case int:
		C.lua_pushnumber(l.State, C.lua_Number(C.longlong(value)))
	case int8:
		C.lua_pushnumber(l.State, C.lua_Number(C.longlong(value)))
	case int16:
		C.lua_pushnumber(l.State, C.lua_Number(C.longlong(value)))
	case int32:
		C.lua_pushnumber(l.State, C.lua_Number(C.longlong(value)))
	case int64:
		C.lua_pushnumber(l.State, C.lua_Number(C.longlong(value)))
	case uint:
		C.lua_pushnumber(l.State, C.lua_Number(C.ulonglong(value)))
	case uint8:
		C.lua_pushnumber(l.State, C.lua_Number(C.ulonglong(value)))
	case uint16:
		C.lua_pushnumber(l.State, C.lua_Number(C.ulonglong(value)))
	case uint32:
		C.lua_pushnumber(l.State, C.lua_Number(C.ulonglong(value)))
	case uint64:
		C.lua_pushnumber(l.State, C.lua_Number(C.ulonglong(value)))
	case float32:
		C.lua_pushnumber(l.State, C.lua_Number(C.double(value)))
	case float64:
		C.lua_pushnumber(l.State, C.lua_Number(C.double(value)))
	case unsafe.Pointer:
		C.lua_pushlightuserdata(l.State, value)
	default:
		// not basic types, use reflect
		switch valueType := reflect.TypeOf(v); valueType.Kind() {
		case reflect.Func:
			// function
			if valueType.IsVariadic() {
				return fmt.Errorf("variadic function is not supported, %s", name)
			}
			function := &_Function{
				name:      name,
				lua:       l,
				fun:       v,
				funcType:  valueType,
				funcValue: reflect.ValueOf(v),
				argc:      valueType.NumIn(),
			}
			funcsLock.Lock()
			funcs = append(funcs, function)
			id := len(funcs) - 1
			funcsLock.Unlock()
			C.push_go_func(l.State, C.int64_t(id))
		case reflect.Slice:
			value := reflect.ValueOf(v)
			length := value.Len()
			C.lua_createtable(l.State, C.int(length), 0)
			for i := 0; i < length; i++ {
				C.lua_pushnumber(l.State, C.lua_Number(i+1))
				err := l.pushGoValue(value.Index(i).Interface(), "")
				if err != nil {
					return err
				}
				C.lua_settable(l.State, -3)
			}
		case reflect.Ptr:
			C.lua_pushlightuserdata(l.State, unsafe.Pointer(reflect.ValueOf(v).Pointer()))
		default:
			// unknown type
			return fmt.Errorf("unsupported type %v", v)
		}
	}
	return nil
}

func (l *Lua) getStackTraceback() string {
	C.lua_getfield(l.State, C.LUA_GLOBALSINDEX, cstr("debug"))
	C.lua_getfield(l.State, -1, cstr("traceback"))
	C.lua_call(l.State, 0, 1)
	return C.GoString(C.lua_tolstring(l.State, -1, nil))
}

//export invokeGoFunc
func invokeGoFunc(state *C.lua_State) int {
	id := C.lua_tointeger(state, C.LUA_GLOBALSINDEX-1)
	funcsLock.RLock()
	function := funcs[id]
	funcsLock.RUnlock()
	// fast paths
	switch f := function.fun.(type) {
	case func():
		f()
		return 0
	}
	// check args
	argc := C.lua_gettop(state)
	if int(argc) != function.argc {
		// Lua.Eval will check err
		function.lua.err = fmt.Errorf("CALL ERROR: number of arguments not match: %s\n%s",
			function.name, function.lua.getStackTraceback())
		return 0
	}
	// prepare args
	var args []reflect.Value
	for i := C.int(1); i <= argc; i++ {
		goValue, err := function.lua.toGoValue(i, function.funcType.In(int(i-1)))
		if err != nil {
			function.lua.err = fmt.Errorf("CALL ERROR: toGoValue error: %v\n%s",
				err, function.lua.getStackTraceback())
			return 0
		}
		if goValue != nil {
			args = append(args, *goValue)
		} else {
			args = append(args, reflect.Zero(function.funcType.In(int(i-1))))
		}
	}
	// call and returns
	returnValues := function.funcValue.Call(args)
	for _, v := range returnValues {
		function.lua.pushGoValue(v.Interface(), "")
	}
	return len(returnValues)
}

// Peval evaluates a piece of lua code. no panic when error occur.
func (l *Lua) Peval(code string, envs ...interface{}) (returns []interface{}, err error) {
	defer C.lua_settop(l.State, 0)
	C.push_errfunc(l.State)
	curTop := C.lua_gettop(l.State)
	// parse
	cCode := C.CString(code)
	defer C.free(unsafe.Pointer(cCode))
	if ret := C.luaL_loadstring(l.State, cCode); ret != 0 { // load error
		return nil, fmt.Errorf("LOAD ERROR: %s", C.GoString(C.lua_tolstring(l.State, -1, nil)))
	}
	// env
	if len(envs) > 0 {
		if len(envs)%2 != 0 {
			return nil, fmt.Errorf("number of arguments not match")
		}
		C.lua_createtable(l.State, 0, 0)
		for i := 0; i < len(envs); i += 2 {
			name, ok := envs[i].(string)
			if !ok {
				return nil, fmt.Errorf("name must be string, not %v", envs[i])
			}
			C.lua_pushstring(l.State, cstr(name))
			err := l.pushGoValue(envs[i+1], name)
			if err != nil {
				return nil, err
			}
			C.lua_rawset(l.State, -3)
		}
		// set env
		C.set_eval_env(l.State)
	}
	// call
	l.err = nil
	if ret := C.lua_pcall(l.State, 0, C.LUA_MULTRET, -2); ret != 0 {
		// error occured
		return nil, fmt.Errorf("CALL ERROR: %s", C.GoString(C.lua_tolstring(l.State, -1, nil)))
	} else if l.err != nil { // error raise by invokeGoFunc
		return nil, l.err
	} else {
		// return values
		nReturn := C.lua_gettop(l.State) - curTop
		returns = make([]interface{}, int(nReturn))
		for i := C.int(0); i < nReturn; i++ {
			value, err := l.toGoValue(-1-i, interfaceType)
			if err != nil {
				return nil, err
			}
			if value != nil {
				returns[int(nReturn-1-i)] = value.Interface()
			} else {
				returns[int(nReturn-1-i)] = nil
			}
		}
	}
	return
}

// Eval evaluates a piece of lua code. panic if error occur.
func (l *Lua) Eval(code string, envs ...interface{}) []interface{} {
	ret, err := l.Peval(code, envs...)
	if err != nil {
		panic(err)
	}
	return ret
}

var stringType = reflect.TypeOf("")
var intType = reflect.TypeOf(int(0))
var floatType = reflect.TypeOf(float64(0))
var boolType = reflect.TypeOf(true)
var interfaceType = reflect.TypeOf((*interface{})(nil)).Elem()

func (l *Lua) toGoValue(i C.int, paramType reflect.Type) (ret *reflect.Value, err error) {
	luaType := C.lua_type(l.State, i)
	paramKind := paramType.Kind()
	switch paramKind {
	case reflect.Bool:
		if luaType != C.LUA_TBOOLEAN {
			err = fmt.Errorf("not a boolean")
			return
		}
		v := reflect.ValueOf(C.lua_toboolean(l.State, i) == C.int(1))
		ret = &v
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not an integer")
			return
		}
		v := reflect.New(paramType).Elem()
		v.SetInt(int64(C.lua_tointeger(l.State, i)))
		ret = &v
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not a unsigned")
			return
		}
		v := reflect.New(paramType).Elem()
		v.SetUint(uint64(C.lua_tointeger(l.State, i)))
		ret = &v
	case reflect.Float32, reflect.Float64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not a float")
			return
		}
		v := reflect.New(paramType).Elem()
		v.SetFloat(float64(C.lua_tonumber(l.State, i)))
		ret = &v
	case reflect.Interface:
		switch paramType {
		case interfaceType:
			switch luaType {
			case C.LUA_TNUMBER:
				v := reflect.New(floatType).Elem() // always return float64 for interface{}
				v.SetFloat(float64(C.lua_tonumber(l.State, i)))
				ret = &v
			case C.LUA_TSTRING:
				v := reflect.New(stringType).Elem()
				v.SetString(C.GoString(C.lua_tolstring(l.State, i, nil)))
				ret = &v
			case C.LUA_TLIGHTUSERDATA, C.LUA_TUSERDATA:
				v := reflect.ValueOf(C.lua_touserdata(l.State, i))
				ret = &v
			case C.LUA_TBOOLEAN:
				v := reflect.New(boolType).Elem()
				v.SetBool(C.lua_toboolean(l.State, i) == C.int(1))
				ret = &v
			case C.LUA_TNIL:
				ret = nil
			default:
				err = fmt.Errorf("unsupported type %s for interface{}", luaTypeName(luaType))
				return
			}
		default:
			err = fmt.Errorf("only interface{} is supported, no %v", paramType)
			return
		}
	case reflect.String:
		if luaType != C.LUA_TSTRING {
			err = fmt.Errorf("not a string")
			return
		}
		v := reflect.New(paramType).Elem()
		v.SetString(C.GoString(C.lua_tolstring(l.State, i, nil)))
		ret = &v
	case reflect.Slice:
		switch luaType {
		case C.LUA_TSTRING:
			v := reflect.New(paramType).Elem()
			cstr := C.lua_tolstring(l.State, i, nil)
			v.SetBytes(C.GoBytes(unsafe.Pointer(cstr), C.int(C.strlen(cstr))))
			ret = &v
		case C.LUA_TTABLE:
			v := reflect.MakeSlice(paramType, 0, 0)
			C.lua_pushnil(l.State)
			elemType := paramType.Elem()
			for C.lua_next(l.State, i) != 0 {
				elemValue, e := l.toGoValue(-1, elemType)
				if e != nil {
					err = e
					return
				}
				// there is no nil value in lua table so elemValue will never be nil
				v = reflect.Append(v, *elemValue)
				C.lua_settop(l.State, -2)
				ret = &v
			}
		default:
			err = fmt.Errorf("wrong slice argument")
			return
		}
	case reflect.Ptr:
		if luaType != C.LUA_TLIGHTUSERDATA {
			err = fmt.Errorf("not a pointer")
			return
		}
		p := C.lua_topointer(l.State, i)
		v := reflect.NewAt(paramType, unsafe.Pointer(&p)).Elem()
		ret = &v
	case reflect.Map:
		if luaType != C.LUA_TTABLE {
			err = fmt.Errorf("not a map")
			return
		}
		v := reflect.MakeMap(paramType)
		C.lua_pushnil(l.State)
		keyType := paramType.Key()
		elemType := paramType.Elem()
		for C.lua_next(l.State, i) != 0 {
			keyValue, e := l.toGoValue(-2, keyType)
			if e != nil {
				err = e
				return
			}
			// table has no nil key so keyValue will not be nil
			elemValue, e := l.toGoValue(-1, elemType)
			if e != nil {
				err = e
				return
			}
			// table has no nil value so elemValue will not be nil
			v.SetMapIndex(*keyValue, *elemValue)
			C.lua_settop(l.State, -2)
		}
		ret = &v
	case reflect.UnsafePointer:
		v := reflect.ValueOf(C.lua_topointer(l.State, i))
		ret = &v
	default:
		err = fmt.Errorf("unsupported toGoValue type %v", paramType)
		return
	}
	return
}

// Pcall calls a lua function. no panic
func (l *Lua) Pcall(fullname string, args ...interface{}) (returns []interface{}, err error) {
	C.push_errfunc(l.State)
	curTop := C.lua_gettop(l.State)
	// get function
	path := strings.Split(fullname, ".")
	for i, name := range path {
		if i == 0 {
			C.lua_getfield(l.State, C.LUA_GLOBALSINDEX, cstr(name))
		} else {
			if C.lua_type(l.State, -1) != C.LUA_TTABLE {
				return nil, fmt.Errorf("%s is not a function", fullname)
			}
			C.lua_pushstring(l.State, cstr(name))
			C.lua_gettable(l.State, -2)
			C.lua_remove(l.State, -2) // remove table
		}
	}
	if C.lua_type(l.State, -1) != C.LUA_TFUNCTION {
		return nil, fmt.Errorf("%s is not a function", fullname)
	}
	// args
	for _, arg := range args {
		l.pushGoValue(arg, "")
	}
	// call
	l.err = nil
	if ret := C.lua_pcall(l.State, C.int(len(args)), C.LUA_MULTRET, C.int(-(len(args))-2)); ret != 0 {
		// error occured
		return nil, fmt.Errorf("CALL ERROR: %s", C.GoString(C.lua_tolstring(l.State, -1, nil)))
	} else if l.err != nil { // error raise by invokeGoFunc
		return nil, l.err
	} else {
		// return values
		nReturn := C.lua_gettop(l.State) - curTop
		returns = make([]interface{}, int(nReturn))
		for i := C.int(0); i < nReturn; i++ {
			value, err := l.toGoValue(-1-i, interfaceType)
			if err != nil {
				return nil, err
			}
			returns[int(nReturn-1-i)] = value.Interface()
		}
	}
	return
}

// Call calls a lua function. panic if error
func (l *Lua) Call(fullname string, args ...interface{}) []interface{} {
	ret, err := l.Pcall(fullname, args...)
	if err != nil {
		panic(err)
	}
	return ret
}

// Close close the lua vm
func (l *Lua) Close() {
	C.lua_close(l.State)
}

var cstrs = make(map[string]*C.char)
var cstrsLock sync.RWMutex

func cstr(str string) *C.char {
	cstrsLock.RLock()
	c, ok := cstrs[str]
	cstrsLock.RUnlock()
	if ok {
		return c
	}
	c = C.CString(str)
	cstrsLock.Lock()
	cstrs[str] = c
	cstrsLock.Unlock()
	return c
}

func luaTypeName(t C.int) (ret string) {
	switch t {
	case C.LUA_TNIL:
		ret = "NIL"
	case C.LUA_TBOOLEAN:
		ret = "BOOLEAN"
	case C.LUA_TLIGHTUSERDATA:
		ret = "LIGHTUSERDATA"
	case C.LUA_TNUMBER:
		ret = "NUMBER"
	case C.LUA_TSTRING:
		ret = "STRING"
	case C.LUA_TTABLE:
		ret = "TABLE"
	case C.LUA_TFUNCTION:
		ret = "FUNCTION"
	case C.LUA_TUSERDATA:
		ret = "USERDATA"
	case C.LUA_TTHREAD:
		ret = "THREAD"
	}
	return
}

func coverLuaTypeName() {
	luaTypeName(C.LUA_TNIL)
	luaTypeName(C.LUA_TBOOLEAN)
	luaTypeName(C.LUA_TLIGHTUSERDATA)
	luaTypeName(C.LUA_TNUMBER)
	luaTypeName(C.LUA_TSTRING)
	luaTypeName(C.LUA_TTABLE)
	luaTypeName(C.LUA_TFUNCTION)
	luaTypeName(C.LUA_TUSERDATA)
	luaTypeName(C.LUA_TTHREAD)
}
