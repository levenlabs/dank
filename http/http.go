package http

import (
	"bitbucket.org/levenlabs/validator"
	"fmt"
	"github.com/mitchellh/mapstructure"
	. "net/http"
	"reflect"
	"strings"
)

var typeOfResponseWriter = reflect.TypeOf(ResponseWriter(nil)).Elem()
var typeOfRequest = reflect.TypeOf((*Request)(nil)).Elem()
var typeOfInt = reflect.TypeOf(int(0)).Elem()
var typeOfError = reflect.TypeOf(error(nil)).Elem()

func methodInList(m string, l []string) bool {
	for _, v := range l {
		if v == m {
			return true
		}
	}
	return false
}

// wrapHandler takes a handler function and for each request, it rejects
// unaccepted methods, converts the query args to the function's args pointer
// and then validates those args.
//
// If the method returns a non-nil error, then the error is returned if its an
// instance of PublicError, otherwise a generic "Internal Error" is sent back
// to the client. If a status code of 0 is returned, then if error is nil, a 500
// is sent and otherwise a 200 is sent.
func WrapHandler(f interface{}, methods ...string) func(ResponseWriter, *Request) {
	fnVal := reflect.ValueOf(f)
	if fnVal.Kind() != reflect.Func {
		panic("http: invalid func passed to wrapHandler")
	}
	fnType := reflect.TypeOf(f)
	if fnType.NumIn() != 3 {
		panic("http: invalid number of args in funcs passed to wrapHandler")
	}
	if fnType.NumOut() != 2 {
		panic("http: invalid number of returns in funcs passed to wrapHandler")
	}
	if fnType.In(0).Elem() != typeOfResponseWriter {
		panic("http: invalid 1st arg in func passed to wrapHandler")
	}
	if fnType.In(1).Elem() != typeOfRequest {
		panic("http: invalid 2nd arg in func passed to wrapHandler")
	}
	argsType := fnType.In(2).Elem()
	if argsType.Kind() != reflect.Ptr {
		panic("http: invalid 3rd arg in func passed to wrapHandler")
	}
	if fnType.Out(0).Elem() != typeOfInt {
		panic("http: invalid 1st return in func passed to wrapHandler")
	}
	if fnType.Out(1).Elem() != typeOfError {
		panic("http: invalid 2nd return in func passed to wrapHandler")
	}
	return func(w ResponseWriter, r *Request) {
		var code int
		var err error
		// first check the method
		if !methodInList(r.Method, methods) {
			code = StatusMethodNotAllowed
			err = fmt.Errorf(
				"http: %s method required, received %s",
				strings.Join(methods, ","),
				r.Method,
			)
		} else {
			args := reflect.New(argsType)
			argsi := args.Interface()
			err = mapstructure.Decode(r.URL.Query(), argsi)
			if err == nil {
				err = validator.Validate(argsi)
			}
			// if we ran into error with validate or mapstructure, invalid args
			if err != nil {
				code = StatusBadRequest
				err = fmt.Errorf("invalid arguments sent: %s", err.Error())
			} else {
				//returns (int, error)
				resVals := fnVal.Call([]reflect.Value{
					reflect.ValueOf(w),
					reflect.ValueOf(r),
					args,
				})
				code = int(resVals[0].Int())
				if errInter := resVals[1].Interface(); errInter != nil {
					err = errInter.(error)
				}
			}
		}
		if err != nil {
			//todo: look for public errors
			if code == 0 {
				code = StatusInternalServerError
			}
			w.WriteHeader(code)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "http: ", err.Error())
			return
		}
	}
}
