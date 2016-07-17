package util_test

import (
	"bytes"
	util "github.com/divtxt/raft/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const (
	test_jsonfile = "test_jsonfile.json"
)

type fooStruct struct {
	BarBaz int `json:"barBaz"`
}

func TestJsonFileSafeWrite(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(wd, test_jsonfile)

	foo := fooStruct{101}

	err = util.SafeWriteJsonToFile(&foo, filename)
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

func TestJsonFileRead(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(wd, test_jsonfile)

	// Read non-existent file
	err = os.Remove(filename)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var foo fooStruct = fooStruct{60}
	err = util.ReadJsonFromFile(filename, &foo)
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if foo.BarBaz != 60 {
		t.Fatal(foo)
	}

	// Read bad file
	var data []byte = []byte("BAD-JSON")
	err = ioutil.WriteFile(filename, data, 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = util.ReadJsonFromFile(filename, &foo)
	if err.Error() != "invalid character 'B' looking for beginning of value" {
		t.Fatal(err)
	}
	if foo.BarBaz != 60 {
		t.Fatal(foo)
	}

	// Read good file
	data = []byte("{\"barBaz\": 201}")
	err = ioutil.WriteFile(filename, data, 0666)
	if err != nil {
		t.Fatal(err)
	}
	err = util.ReadJsonFromFile(filename, &foo)
	if err != nil {
		t.Fatal(err)
	}
	if foo.BarBaz != 201 {
		t.Fatal(foo)
	}
}
