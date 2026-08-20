// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"flag"
	"testing"

	"github.com/google/note-maps/note"
	"github.com/google/subcommands"
)

// captureDB implements note.Database, recording the operations that set passes
// to Patch so a test can verify that a parsed note is actually written.
type captureDB struct {
	patched []note.Operation
}

func (db *captureDB) IsolatedRead(f func(r note.FindLoader) error) error {
	return f(capturePatcher{db})
}

func (db *captureDB) IsolatedWrite(f func(rw note.FindLoadPatcher) error) error {
	return f(capturePatcher{db})
}

func (db *captureDB) Close() error { return nil }

type capturePatcher struct{ db *captureDB }

func (p capturePatcher) Find(*note.Query) ([]note.GraphNote, error) { return nil, nil }
func (p capturePatcher) Load(ids []note.ID) ([]note.GraphNote, error) {
	ns := make([]note.GraphNote, len(ids))
	for i, id := range ids {
		if id == note.EmptyID {
			return nil, note.InvalidID
		}
		ns[i] = note.EmptyNote(id)
	}
	return ns, nil
}
func (p capturePatcher) Patch(ops []note.Operation) error {
	p.db.patched = ops
	return nil
}

// TestSetCmdWritesNote verifies that the set command turns the note it parses
// from YAML into operations that are actually patched into the database,
// rather than patching an empty set of operations.
func TestSetCmdWritesNote(t *testing.T) {
	db := &captureDB{}
	var output bytes.Buffer
	cmd := &setCmd{cfg: &Config{
		overrideDb: db,
		input:      bytes.NewBufferString("note: &42\n- is: hello\n"),
		output:     &output,
	}}
	exit := cmd.Execute(context.Background(), flag.NewFlagSet("set", flag.ContinueOnError))
	if exit != subcommands.ExitSuccess {
		t.Fatalf("set exited with %v, want success", exit)
	}
	if len(db.patched) == 0 {
		t.Fatal("set did not write any operations to the database")
	}
	var (
		value    string
		foundSet bool
	)
	for _, op := range db.patched {
		switch o := op.(type) {
		case note.OpSetValue:
			if o.GetID() == note.ID("42") {
				value = o.Lexical
				foundSet = true
			}
		}
	}
	if !foundSet {
		t.Fatalf("expected an operation setting the value of note %q, got %v",
			"42", db.patched)
	}
	if value != "hello" {
		t.Errorf("expected note value to be %q, got %q", "hello", value)
	}
}

// TestSetCmdWritesContents verifies that set also writes the contents of a
// parsed note, including their own values.
func TestSetCmdWritesContents(t *testing.T) {
	db := &captureDB{}
	var output bytes.Buffer
	cmd := &setCmd{cfg: &Config{
		overrideDb: db,
		input: bytes.NewBufferString(
			"note: &42\n- is: hello\n- &43\n  - is: world\n"),
		output: &output,
	}}
	exit := cmd.Execute(context.Background(), flag.NewFlagSet("set", flag.ContinueOnError))
	if exit != subcommands.ExitSuccess {
		t.Fatalf("set exited with %v, want success", exit)
	}
	values := make(map[note.ID]string)
	contents := make(map[note.ID][]note.ID)
	for _, op := range db.patched {
		switch o := op.(type) {
		case note.OpSetValue:
			values[o.GetID()] = o.Lexical
		case note.OpContentDelta:
			contents[o.GetID()] = note.IDSlice{}.Apply(o.IDSliceOps)
		}
	}
	if values[note.ID("42")] != "hello" {
		t.Errorf("expected note 42 to have value %q, got %q", "hello", values[note.ID("42")])
	}
	if values[note.ID("43")] != "world" {
		t.Errorf("expected note 43 to have value %q, got %q", "world", values[note.ID("43")])
	}
	if got := contents[note.ID("42")]; len(got) != 1 || got[0] != note.ID("43") {
		t.Errorf("expected note 42 to contain note 43, got %v", got)
	}
}
