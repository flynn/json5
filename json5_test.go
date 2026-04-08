package json5

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	"github.com/robertkrimen/otto"
)

type ErrorSpec struct {
	At           int64
	LineNumber   int
	ColumnNumber int
	Message      string
}

func TestJSON5Decode(t *testing.T) {
	filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			t.Errorf("error reading file: %s", err)
			return nil
		}

		parseJSON5 := func() (interface{}, error) {
			var res interface{}
			return res, Unmarshal(data, &res)
		}
		parseJSON := func() (interface{}, error) {
			var res interface{}
			return res, json.Unmarshal(data, &res)
		}
		parseES5 := func() (interface{}, error) {
			vm := otto.New()
			_, err := vm.Run("x=" + string(data))
			if err != nil {
				return nil, err
			}
			v, err := vm.Get("x")
			if err != nil {
				return nil, err
			}
			return v.Export()
		}

		t.Logf("file: %s", path)

		switch filepath.Ext(path) {
		case ".json":
			jd, err := parseJSON()
			if err != nil {
				t.Errorf("%s\nunexpected error from json decoder: %s", path, err)
				return nil
			}
			j5d, err := parseJSON5()
			if err != nil {
				t.Errorf("%s\nunexpected error from json5 decoder: %s", path, err)
				return nil
			}
			if diff := pretty.Compare(jd, j5d); diff != "" {
				t.Errorf("%s\ndata is not equal\n%s", path, diff)
				return nil
			}
		case ".json5":
			if _, err := parseJSON(); err == nil {
				t.Errorf("%s\nexpected JSON parsing to fail", path)
				return nil
			}
			es5d, err := parseES5()
			if err != nil {
				t.Errorf("%s\nunexpected error from ES5 decoder: %s", path, err)
				return nil
			}
			j5d, err := parseJSON5()
			if err != nil {
				t.Errorf("%s\nunexpected error from json5 decoder: %s", path, err)
				return nil
			}
			if diff := pretty.Compare(j5d, es5d); diff != "" {
				t.Errorf("%s\ndata is not equal\n%s", path, diff)
				return nil
			}
		case ".js":
			if _, err := parseJSON(); err == nil {
				t.Errorf("%s\nexpected JSON parsing to fail", path)
				return nil
			}
			if _, err := parseES5(); err != nil {
				t.Errorf("%s\nunexected error from ES5 decoder: %s", path, err)
				return nil
			}
			if _, err := parseJSON5(); err == nil {
				t.Errorf("%s\nexpected JSON5 parsing to fail", path)
				return nil
			}
		case ".txt":
			var expectedErr *ErrorSpec
			specName := path[:len(path)-4] + ".errorSpec"
			specFile, err := os.Open(specName)
			if err != nil && !os.IsNotExist(err) {
				t.Errorf("%s\nerror trying to open errorSpec file %s: %s", path, specName, err)
				return nil
			}
			if specFile != nil {
				expectedErr = &ErrorSpec{}
				if err := NewDecoder(specFile).Decode(expectedErr); err != nil {
					specFile.Close()
					t.Errorf("%s\nerror decoding %s: %s", path, specName, err)
					return nil
				}
				specFile.Close() // not using defer because we're in a long loop
			}
			_, err = parseJSON5()
			if err == nil {
				t.Errorf("%s\nexpected JSON5 parsing to fail", path)
				return nil
			}
			//if expectedErr != nil && !matchedError(err, expectedErr) {
			if !matchedError(err, expectedErr) {
				t.Errorf("%s\nexpected JSON5 error %+v\nbut got: %+v", path, expectedErr, err)
				return nil
			}
		}

		return nil
	})
}

func matchedError(err error, expected *ErrorSpec) bool {
	var se *SyntaxError
	if errors.As(err, &se) {
		return se.msg == expected.Message && se.Offset == expected.At
	}
	var ute *UnmarshalTypeError
	if errors.As(err, &ute) {
		return ute.message() == expected.Message && ute.Offset == expected.At
	}
	return false
}

// The tests below this comment were found with go-fuzz

func TestQuotedQuote(t *testing.T) {
	var v struct {
		E string
	}
	if err := Unmarshal([]byte(`{e:"'"}`), &v); err != nil {
		t.Error(err)
	}
	if v.E != "'" {
		t.Errorf(`expected "'", got %q`, v.E)
	}
}

func TestInvalidNewline(t *testing.T) {
	expected := "json: invalid character '\\n' in string literal at offset 8"
	var v interface{}
	if err := Unmarshal([]byte("{a:'\\\r0\n'}"), &v); err == nil || err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err)
	}
}
