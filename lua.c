#include "lua.h"

extern int invokeGoFunc(lua_State*);

void push_go_func(lua_State *l, void* func) {
  lua_pushlightuserdata(l, func);
  lua_pushcclosure(l, (lua_CFunction)invokeGoFunc, 1);
}
