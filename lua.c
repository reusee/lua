#include "lua.h"

extern int invokeGoFunc(lua_State*);

void push_go_func(lua_State *l, void* func) {
  lua_pushlightuserdata(l, func);
  lua_pushcclosure(l, (lua_CFunction)invokeGoFunc, 1);
}

int traceback(lua_State *l) {
  lua_getfield(l, LUA_GLOBALSINDEX, "debug");
  lua_getfield(l, -1, "traceback");
  lua_call(l, 0, 1);
  return 1;
}

void push_errfunc(lua_State *l) {
  lua_pushcfunction(l, traceback);
}
