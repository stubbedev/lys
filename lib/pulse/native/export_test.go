package native

import (
	"reflect"
	"runtime"
)

func RoutineExit() string {
	return runtime.FuncForPC(reflect.ValueOf((*routine).run).Pointer()).Name()
}
