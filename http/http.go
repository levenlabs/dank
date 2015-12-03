package http

import (
	"fmt"
	"github.com/levenlabs/go-llog"
	"github.com/levenlabs/golib/rpcutil"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/validator.v2"
	. "net/http"
	"net/url"
	"reflect"
	"runtime"
	"strings"
)

//store the common types needed to wrapHandler
var typeOfResponseWriter = reflect.TypeOf((*ResponseWriter)(nil)).Elem()
var typeOfPtrRequest = reflect.TypeOf((*Request)(nil))
var typeOfInt = reflect.TypeOf(int(0))
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// strInList determines if the string m is in the list l
func strInList(m string, l []string) bool {
	for _, v := range l {
		if v == m {
			return true
		}
	}
	return false
}

// firstQueryVals takes a url and returns a map taking only the first value of
// each query param sent
func firstQueryVals(u *url.URL) map[string]string {
	m := u.Query()
	dst := make(map[string]string)
	for k, v := range m {
		if len(v) > 0 {
			dst[k] = v[0]
		}
	}
	return dst
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
		panic("http: invalid number of args in func passed to wrapHandler")
	}
	if fnType.NumOut() != 2 {
		panic("http: invalid number of returns in func passed to wrapHandler")
	}
	if fnType.In(0) != typeOfResponseWriter {
		panic("http: invalid 1st arg in func passed to wrapHandler")
	}
	if fnType.In(1) != typeOfPtrRequest {
		panic("http: invalid 2nd arg in func passed to wrapHandler")
	}
	argsType := fnType.In(2)
	if argsType.Kind() != reflect.Ptr {
		panic("http: invalid 3rd arg in func passed to wrapHandler")
	}
	argsElem := argsType.Elem()
	if fnType.Out(0) != typeOfInt {
		panic("http: invalid 1st return in func passed to wrapHandler")
	}
	if fnType.Out(1) != typeOfError {
		panic("http: invalid 2nd return in func passed to wrapHandler")
	}
	fnName := runtime.FuncForPC(fnVal.Pointer()).Name()
	return func(w ResponseWriter, r *Request) {
		kv := rpcutil.RequestKV(r)
		kv["handler"] = fnName
		llog.Debug("Received HTTP request", kv)

		var code int
		var err error
		// first check the method
		if !strInList(r.Method, methods) {
			code = StatusMethodNotAllowed
			err = fmt.Errorf(
				"http: %s method required, received %s",
				strings.Join(methods, ","),
				r.Method,
			)
		} else {
			args := reflect.New(argsElem)
			argsi := args.Interface()
			err = mapstructure.Decode(firstQueryVals(r.URL), argsi)
			if err == nil {
				err = validator.Validate(argsi)
			}
			// if we ran into error with validate or mapstructure, invalid args
			if err != nil {
				code = StatusBadRequest
				err = fmt.Errorf("invalid arguments sent: %s", err.Error())
			} else {
				// accepts (http.ResponseWriter, *http.Request, interface{})
				// returns (int, error)
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
			fmt.Fprint(w, err.Error())
			kv["error"] = err
		} else if code != 0 {
			w.WriteHeader(code)
		}
		kv["code"] = code
		llog.Debug("Responded to HTTP request", kv)
	}
}
