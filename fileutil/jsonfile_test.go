package fileutil_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/divtxt/raft/fileutil"
)

const (
	test_jsonfile = "test_jsonfile.json"
)

type fooStruct struct {
	BarBaz int `json:"barBaz"`
}

func TestAtomicJsonFile_Write(t *testing.T) {
	ajf := fileutil.NewAtomicJsonFile(test_jsonfile)

	foo := fooStruct{101}

	err := ajf.Write(&foo)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadFile(test_jsonfile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(data, []byte("{\"barBaz\":101}")) != 0 {
		t.Fatal()
	}
}

func TestAtomicJsonFile_Read(t *testing.T) {
	ajf := fileutil.NewAtomicJsonFile(test_jsonfile)

	// Read non-existent file
	err := os.Remove(test_jsonfile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var foo fooStruct = fooStruct{60}
	err = ajf.Read(&foo)
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if foo.BarBaz != 60 {
		t.Fatal(foo)
	}

	// Read bad file
	var data []byte = []byte("BAD-JSON")
	err = ioutil.WriteFile(test_jsonfile, data, 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ajf.Read(&foo)
	if err.Error() != "invalid character 'B' looking for beginning of value" {
		t.Fatal(err)
	}
	if foo.BarBaz != 60 {
		t.Fatal(foo)
	}

	// Read good file
	data = []byte("{\"barBaz\": 201}")
	err = ioutil.WriteFile(test_jsonfile, data, 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = ajf.Read(&foo)
	if err != nil {
		t.Fatal(err)
	}
	if foo.BarBaz != 201 {
		t.Fatal(foo)
	}
}
