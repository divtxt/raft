package raft_extra

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
)

// "Atomically" write the given object as json to the given file.
//
// Writes to a file named "filename.bak", and then renames it to the given
// name. This ensures that the file is always valid json.
//
func SafeWriteJsonToFile(v interface{}, filename string) error {
	filenamebak := strings.Join([]string{filename, "bak"}, ".")
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filenamebak, data, 0666)
	if err != nil {
		return err
	}
	return os.Rename(filenamebak, filename)
}

// Read the given file as json into the given object.
func ReadJsonFromFile(filename string, v interface{}) error {
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
