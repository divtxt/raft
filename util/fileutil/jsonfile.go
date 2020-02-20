package fileutil

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

// "Atomically" write the JSON encoding of v to the given file.
//
// The "atomic" write is done by writing to a file named "<filename>.new"
// and then renaming it to "<filename>".
//
// Assuming all writers use this function to write the file, this method ensures
// that readers will always see a "<filename>" that contains valid complete json
// since empty or partially written files, due to in progress or crashed writes, will
// never affect the original file.
//
// However, because this uses the same temp file name for all writers, this is
// NOT concurrent safe. (One writer may rename another's partially written file)
//
func WriteJsonAtomic(v interface{}, filename string) error {
	filenameBak := strings.Join([]string{filename, "bak"}, ".")
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filenameBak, data, 0666)
	if err != nil {
		return err
	}
	return os.Rename(filenameBak, filename)
}

// ReadJson parses the JSON-encoded data in the given file and
// stores the result in the value pointed to by v.
//
func ReadJson(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}
