package fileutil

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// AtomicJsonFile is a simple API to atomically read and write a json file.
//
// The atomic write is done by writing to a file named "<filename>.new"
// and then renaming it to "<filename>" after the write is complete.
//
// Assuming all writers use the same method to write the file, this method ensures
// that readers will always see a "<filename>" that contains valid complete json
// since empty or partially written files, due to in progress or crashed writes, will
// never affect the original file.
//
// However, because this uses the same temp file name for all writers, this is
// NOT safe for concurrent writes. (One writer may rename another's partially written file)
//
type AtomicJsonFile interface {
	// Read the JSON-encoded data in the wrapped file and
	// stores the result in the value pointed to by v.
	Read(v interface{}) error

	// Write the JSON encoding of v to the wrapped file.
	Write(v interface{}) error
}

type atomicJsonFile struct {
	filename    string
	filenameTmp string
}

// Create an AtomicJsonFile for the given file.
func NewAtomicJsonFile(filename string) AtomicJsonFile {
	return &atomicJsonFile{filename, filename + ".new"}
}

func (ajf *atomicJsonFile) Read(v interface{}) error {
	data, err := ioutil.ReadFile(ajf.filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func (ajf *atomicJsonFile) Write(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(ajf.filenameTmp, data, 0666)
	if err != nil {
		return err
	}
	return os.Rename(ajf.filenameTmp, ajf.filename)
}
