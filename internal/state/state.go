package state

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/kong/kubernetes-testing-framework/pkg/environment"
	"github.com/mitchellh/go-homedir"
)

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------

// State represents the persistent state of the KTF including its active environments, e.t.c.
// TODO: filesystem lock - disallow concurrent writes
type State struct {
	l *sync.RWMutex
	f *os.File

	// Environments is a list of all the test.Environments (FIXME: placeholder) present on the system.
	Environments []environment.Environment `json:"environments,omitempty"`
}

// NewFromFile provides a new *State given an *os.File containing valid contents.
func NewFromFile(f *os.File) (*State, error) {
	s := &State{l: &sync.RWMutex{}, f: f}
	return s, s.Refresh()
}

// Refresh pulls the latest *State from disk.
func (s *State) Refresh() error {
	s.l.Lock()
	defer s.l.Unlock()

	// read the contents of the state file
	b, err := ioutil.ReadAll(s.f)
	if err != nil {
		return err
	}

	// decode the JSON contents
	if err := json.Unmarshal(b, s); err != nil {
		return err
	}

	return nil
}

// CreateEnvironment creates a new testing environment and loads it into the state.
func (s *State) CreateEnvironment(builder *environment.Builder) (environment.Environment, error) {
	return builder.Create()
}

// -----------------------------------------------------------------------------
// State - Helper Functions
// -----------------------------------------------------------------------------

// GetStateFileLocation provides the directory and name of the state file as
// separate strings and validates whether the cache directory exists. If the
// $HOME/.cache directory doesn't exist, it will attempt to create it.
func GetStateFileLocation() (dir, name string, err error) {
	// determine the home directory
	var home string
	home, err = homedir.Dir()
	if err != nil {
		return
	}

	// find the $HOME/.cache dir, create the directory if necessary
	dir, name = filepath.Join(home, ".cache"), "ktf.json"
	err = os.MkdirAll(dir, 0750)
	return
}

// GetStateFile gets an *os.File for the ktf state file, creating the file if necessary.
func GetStateFile() (*os.File, error) {
	// get the full path to the state file
	dir, name, err := GetStateFileLocation()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)

	// attempt to open the state file
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		// create the state file with an empty state if necessary
		if err := ioutil.WriteFile(path, []byte(`{}`), 0640); err != nil {
			return nil, err
		}
		f, err = os.Open(path)
	}

	return f, err
}
