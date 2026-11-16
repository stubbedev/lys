package native

import (
	"reflect"
	"runtime"
)

var RoutineExit = runtime.FuncForPC(reflect.ValueOf((*routine).run).Pointer()).Name()
