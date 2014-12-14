#include "lua.h"
#include <lauxlib.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

extern int invokeGoFunc(lua_State*);

void push_go_func(lua_State *l, int64_t* p) {
  lua_pushlightuserdata(l, p);
  lua_pushcclosure(l, (lua_CFunction)invokeGoFunc, 1);
}

int traceback(lua_State *l) {
  lua_pushstring(l, "\n"); // separate error message and traceback
  lua_getfield(l, LUA_GLOBALSINDEX, "debug");
  lua_getfield(l, -1, "traceback");
  lua_call(l, 0, 1);
  lua_remove(l, -2); // remove debug table from stack
  lua_concat(l, 3); // concat origin error message
  return 1;
}

void push_errfunc(lua_State *l) {
  lua_pushcfunction(l, traceback);
}

lua_State* new_state() {
  lua_State *state = luaL_newstate();
  if (state == NULL) {
    return NULL;
  }
  luaL_openlibs(state);
  return state;
}

char* ensure_name(lua_State *l, char *fullname) {
  char *name, *next, *p = fullname;
  int type;

  lua_getfield(l, LUA_GLOBALSINDEX, "_G");

  name = strtok(fullname, ".");
  next = strtok(NULL, ".");
  while (name != NULL) {
    if (next == NULL) { // variable name
      lua_pushstring(l, name); // push as key
    } else { // namespace
      lua_pushstring(l, name);
      lua_rawget(l, -2);
      type = lua_type(l, -1);
      if (type == LUA_TNIL) { // not exists, create new
        lua_settop(l, -2);
        lua_pushstring(l, name);
        lua_createtable(l, 0, 0);
        lua_rawset(l, -3);
        lua_pushstring(l, name);
        lua_rawget(l, -2);
      } else if (type != LUA_TTABLE) { // not a table
        return "invalid namespace";
      }
    }
    name = next;
    next = strtok(NULL, ".");
  }

  free(p);
  return NULL;
}

void set_eval_env(lua_State *l) {
  // set env's metatable to _G
  lua_createtable(l, 0, 0);
  lua_pushstring(l, "__index");
  lua_getfield(l, LUA_GLOBALSINDEX, "_G");
  lua_rawset(l, -3);
  lua_setmetatable(l, -2);
  // set function env
  lua_setfenv(l, -2);
}
