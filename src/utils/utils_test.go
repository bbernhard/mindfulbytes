package utils

import (
	"testing"
	"time"
	"fmt"
	"runtime"
	"path/filepath"
	"reflect"
)

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// notOk fails the test if an err is nil.
func notOk(tb testing.TB, err error) {
	if err == nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error, expected not nil, but got nil: \033[39m\n\n", filepath.Base(file), line)
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}

func TestReplaceEnglishTimeagoTag(t *testing.T) {
	d := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	out, err := ReplaceTagsInMessage("This picture was created {{timeago}}", d, "en")
	ok(t, err)
	equals(t, out, "This picture was created 11 years ago")
}

func TestReplaceEnglishTimeagoTag1(t *testing.T) {
	d := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	out, err := ReplaceTagsInMessage("This picture was created {{ timeago }}", d, "en")
	ok(t, err)
	equals(t, out, "This picture was created 11 years ago")
}

func TestReplaceGermanTimeagoTag(t *testing.T) {
	d := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	out, err := ReplaceTagsInMessage("Das Foto entstand {{timeago}}", d, "ge")
	ok(t, err)
	equals(t, out, "Das Foto entstand vor 11 Jahren")
}
