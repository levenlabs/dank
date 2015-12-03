package http
import (
	. "net/http"
	"fmt"
	"github.com/levenlabs/dank/upload"
	"github.com/levenlabs/dank/seaweed"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"bitbucket.org/levenlabs/validator"
	"encoding/json"
	"io/ioutil"
)

func init() {
	HandleFunc("/get", wrapHandler(getHandler))
	HandleFunc("/upload", wrapHandler(uploadHandler))
	HandleFunc("/assign", wrapHandler(assignHandler))
	HandleFunc("/verify", wrapHandler(verifyHandler))
}

type GetArgs struct {
	Filename string `json:"filename" mapstructure:"filename" validate:"nonzero"`
}

func getHandler(w ResponseWriter, r *Request, args *GetArgs) {
	if !requireMethod("GET", w, r) {
		return
	}
	//todo: support /get/filname.jpg additionally to ease nginx proxying
	//todo: copy headers from seaweed?
	err := seaweed.Get(args.Filename, w)
	if err != nil {
		writeErrorf(w, StatusInternalServerError, "internal error")
		return
	}
}

func uploadHandler(w ResponseWriter, r *Request, args *upload.Assignment) {
	if !requireMethod("POST", w, r) {
		return
	}
	//todo: handle form uploads
	f, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeErrorf(w, StatusInternalServerError, "internal error")
		return
	}
	err = upload.Upload(args, f)
	if err != nil {
		writeErrorf(w, StatusInternalServerError, "internal error")
		return
	}
	return
}

func assignHandler(w ResponseWriter, r *Request, args *upload.AssignRequest) {
	if !requireMethod("GET", w, r) {
		return
	}
	a, err := upload.Assign(args)
	if err != nil {
		writeErrorf(w, StatusInternalServerError, "internal error")
		return
	}
	js, err := json.Marshal(a)
	if err != nil {
		writeErrorf(w, StatusInternalServerError, "json marshal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func verifyHandler(w ResponseWriter, r *Request, args *upload.Assignment) {
	if !requireMethod("GET", w, r) {
		return
	}
	err := upload.Verify(args)
	if err != nil {
		writeErrorf(w, StatusBadRequest, "invalid filename sent %s", err.Error())
		return
	}
	return
}

func wrapHandler(fn func (ResponseWriter, *Request, interface{})) func(ResponseWriter, *Request) {
	argsType := reflect.TypeOf(fn).In(2).Elem()
	return func(w ResponseWriter, r *Request) {
		args := reflect.New(argsType).Interface()
		err := mapstructure.Decode(r.URL.Query(), args)
		if err != nil {
			writeErrorf(w, StatusBadRequest, "http: invalid arguments sent %s", err.Error())
			return
		}
		err = validator.Validate(args)
		if err != nil {
			writeErrorf(w, StatusBadRequest, "http: invalid arguments sent %s", err.Error())
			return
		}
		fn(w, r, args)
	}
}

func requireMethod(m string, w ResponseWriter, r *Request) bool {
	if r.Method != m {
		writeErrorf(w, StatusMethodNotAllowed, "http: %s method required, received %s", m, r.Method)
		return false
	}
	return true
}

func writeErrorf(w ResponseWriter, status int, msg string, args ...interface{}) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(msg, args...))
}
