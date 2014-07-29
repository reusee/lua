package lua

/*
#include <lua.h>
#include <lualib.h>
#include <lauxlib.h>
#include <string.h>

void push_go_func(lua_State*, void*);

#cgo pkg-config: luajit
*/
import "C"
import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

type Lua struct {
	State *C.lua_State
	funcs []*Function
}

type Function struct {
	name      string
	lua       *Lua
	fun       interface{}
	funcType  reflect.Type
	funcValue reflect.Value
	argc      int
}

func New() (*Lua, error) {
	state := C.luaL_newstate()
	if state == nil {
		return nil, fmt.Errorf("lua newstate")
	}
	C.luaL_openlibs(state)
	lua := &Lua{
		State: state,
	}
	return lua, nil
}

func (l *Lua) Set(args ...interface{}) error {
	if len(args)%2 != 0 {
		return fmt.Errorf("number of arguments not match, check your program.")
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

func (l *Lua) set(fullname string, v interface{}) error {
	// resolve name
	path := strings.Split(fullname, ".")
	name := path[len(path)-1]
	path = path[:len(path)-1]
	if len(path) == 0 {
		path = append(path, "_G")
	}

	// ensure namespaces(table)
	for i, namespace := range path {
		cNamespace := cstr(namespace)
		if i == 0 { // top level namespace
			C.lua_getfield(l.State, C.LUA_GLOBALSINDEX, cNamespace)
			if t := C.lua_type(l.State, -1); t == C.LUA_TNIL { // not exists, create new
				C.lua_settop(l.State, -2)
				C.lua_createtable(l.State, 0, 0)
				C.lua_setfield(l.State, C.LUA_GLOBALSINDEX, cNamespace)
				C.lua_getfield(l.State, C.LUA_GLOBALSINDEX, cNamespace) // set as current namespace
			} else if t != C.LUA_TTABLE { // not a table
				return fmt.Errorf("global %s is not a table", namespace)
			}
		} else { // sub namespace
			C.lua_pushstring(l.State, cNamespace)
			C.lua_rawget(l.State, -2)
			if t := C.lua_type(l.State, -1); t == C.LUA_TNIL { // not exists, create new
				C.lua_settop(l.State, -2)
				C.lua_pushstring(l.State, cNamespace)
				C.lua_createtable(l.State, 0, 0)
				C.lua_rawset(l.State, -3)
				C.lua_pushstring(l.State, cNamespace) // set as current namespace
				C.lua_rawget(l.State, -2)
			} else if t != C.LUA_TTABLE { // not a table
				return fmt.Errorf("namespace %s is not a table", strings.Join(path[:i+1], "."))
			}
		}
	}

	// push name
	cName := cstr(name)
	C.lua_pushstring(l.State, cName)

	// push value
	err := l.pushGoValue(v, fullname)
	if err != nil {
		return err
	}

	// set
	C.lua_rawset(l.State, -3)
	C.lua_settop(l.State, -2) // unload namespace

	return nil
}

func (l *Lua) pushGoValue(v interface{}, name string) error {
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
			function := &Function{
				name:      name,
				lua:       l,
				fun:       v,
				funcType:  valueType,
				funcValue: reflect.ValueOf(v),
				argc:      valueType.NumIn(),
			}
			l.funcs = append(l.funcs, function) // hold reference of func
			C.push_go_func(l.State, unsafe.Pointer(function))
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
		case reflect.Interface:
			err := l.pushGoValue(reflect.ValueOf(v).Elem(), "")
			if err != nil {
				return err
			}
		case reflect.Ptr:
			C.lua_pushlightuserdata(l.State, unsafe.Pointer(reflect.ValueOf(v).Pointer()))
		default:
			// unknown type
			panic(fmt.Sprintf("fixme, %v not handle", v))
		}
	}
	return nil
}

//export invokeGoFunc
func invokeGoFunc(state *C.lua_State) int {
	p := C.lua_touserdata(state, C.LUA_GLOBALSINDEX-1)
	function := (*Function)(p)
	// fast paths
	switch f := function.fun.(type) {
	case func():
		f()
		return 0
	}
	// check args
	argc := C.lua_gettop(state)
	if int(argc) != function.argc {
		//TODO not elegant
		C.lua_pushstring(state,
			C.CString(fmt.Sprintf("%s: number of arguments not match", function.name)))
		C.lua_error(state)
		return 0
	}
	// prepare args
	var args []reflect.Value
	for i := C.int(1); i <= argc; i++ {
		goValue, err := function.lua.toGoValue(i, function.funcType.In(int(i-1)))
		if err != nil {
			//TODO not elegant
			C.lua_pushstring(state,
				C.CString(fmt.Sprintf("%s: toGoValue error %v", err)))
			C.lua_error(state)
		}
		args = append(args, goValue)
	}
	// call and returns
	returnValues := function.funcValue.Call(args)
	for _, v := range returnValues {
		function.lua.pushGoValue(v.Interface(), "")
	}
	return len(returnValues)
}

func (l *Lua) Eval(code string) (returns []interface{}, err error) {
	curTop := C.lua_gettop(l.State)
	cCode := cstr(code)
	if ret := C.luaL_loadstring(l.State, cCode); ret != 0 { // load error
		return nil, fmt.Errorf("%s", C.GoString(C.lua_tolstring(l.State, -1, nil)))
	}
	//TODO set a errfunc to collect traceback?
	if ret := C.lua_pcall(l.State, 0, C.LUA_MULTRET, 0); ret != 0 {
		// error occured
		return nil, fmt.Errorf("%s", C.GoString(C.lua_tolstring(l.State, -1, nil)))
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

var stringType = reflect.TypeOf("")
var intType = reflect.TypeOf(int(0))
var floatType = reflect.TypeOf(float64(0))
var boolType = reflect.TypeOf(true)
var interfaceType = reflect.TypeOf((*interface{})(nil)).Elem()

func (l *Lua) toGoValue(i C.int, paramType reflect.Type) (ret reflect.Value, err error) {
	luaType := C.lua_type(l.State, i)
	paramKind := paramType.Kind()
	switch paramKind {
	case reflect.Bool:
		if luaType != C.LUA_TBOOLEAN {
			err = fmt.Errorf("not a boolean")
			return
		}
		ret = reflect.ValueOf(C.lua_toboolean(l.State, i) == C.int(1))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not an integer")
			return
		}
		ret = reflect.New(paramType).Elem()
		ret.SetInt(int64(C.lua_tointeger(l.State, i)))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not a unsigned")
			return
		}
		ret = reflect.New(paramType).Elem()
		ret.SetUint(uint64(C.lua_tointeger(l.State, i)))
	case reflect.Float32, reflect.Float64:
		if luaType != C.LUA_TNUMBER {
			err = fmt.Errorf("not a float")
			return
		}
		ret = reflect.New(paramType).Elem()
		ret.SetFloat(float64(C.lua_tonumber(l.State, i)))
	case reflect.Interface:
		switch paramType {
		case interfaceType:
			switch luaType {
			case C.LUA_TNUMBER:
				ret = reflect.New(floatType).Elem() // always return float64 for interface{}
				ret.SetFloat(float64(C.lua_tonumber(l.State, i)))
			case C.LUA_TSTRING:
				ret = reflect.New(stringType).Elem()
				ret.SetString(C.GoString(C.lua_tolstring(l.State, i, nil)))
			case C.LUA_TLIGHTUSERDATA:
				ret = reflect.ValueOf(C.lua_topointer(l.State, i))
			case C.LUA_TBOOLEAN:
				ret = reflect.New(boolType).Elem()
				ret.SetBool(C.lua_toboolean(l.State, i) == C.int(1))
			default:
				err = fmt.Errorf("unsupported value for interface{}, %v", paramKind)
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
		ret = reflect.New(paramType).Elem()
		ret.SetString(C.GoString(C.lua_tolstring(l.State, i, nil)))
	case reflect.Slice:
		switch luaType {
		case C.LUA_TSTRING:
			ret = reflect.New(paramType).Elem()
			cstr := C.lua_tolstring(l.State, i, nil)
			ret.SetBytes(C.GoBytes(unsafe.Pointer(cstr), C.int(C.strlen(cstr))))
		case C.LUA_TTABLE:
			ret = reflect.MakeSlice(paramType, 0, 0)
			C.lua_pushnil(l.State)
			elemType := paramType.Elem()
			for C.lua_next(l.State, i) != 0 {
				elemValue, e := l.toGoValue(-1, elemType)
				if e != nil {
					err = e
					return
				}
				ret = reflect.Append(ret, elemValue)
				C.lua_settop(l.State, -2)
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
		ret = reflect.ValueOf(C.lua_topointer(l.State, i))
	case reflect.Map:
		if luaType != C.LUA_TTABLE {
			err = fmt.Errorf("not a map")
			return
		}
		ret = reflect.MakeMap(paramType)
		C.lua_pushnil(l.State)
		keyType := paramType.Key()
		elemType := paramType.Elem()
		for C.lua_next(l.State, i) != 0 {
			keyValue, e := l.toGoValue(-2, keyType)
			if e != nil {
				err = e
				return
			}
			elemValue, e := l.toGoValue(-1, elemType)
			if e != nil {
				err = e
				return
			}
			ret.SetMapIndex(keyValue, elemValue)
			C.lua_settop(l.State, -2)
		}
	case reflect.UnsafePointer:
		ret = reflect.ValueOf(C.lua_topointer(l.State, i))
	default:
		err = fmt.Errorf("unknown argument type %v", paramType)
		return
	}
	return
}

func (l *Lua) Close() {
	C.lua_close(l.State)
}

var cstrs = make(map[string]*C.char)

func cstr(str string) *C.char {
	if c, ok := cstrs[str]; ok {
		return c
	}
	c := C.CString(str)
	cstrs[str] = c
	return c
}
