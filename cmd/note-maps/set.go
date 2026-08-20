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
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/google/note-maps/note"
	"github.com/google/note-maps/note/yaml"
	"github.com/google/subcommands"
)

type setCmd struct {
	cfg *Config
}

func (*setCmd) Name() string     { return "set" }
func (*setCmd) Synopsis() string { return "Set the info about a subject." }
func (*setCmd) Usage() string {
	return `set:
  Import a note in YAML format.
`
}
func (c *setCmd) SetConfig(cfg *Config) { c.cfg = cfg }
func (c *setCmd) SetFlags(f *flag.FlagSet) {
	//f.BoolVar(&c.capitalize, "capitalize", false, "capitalize output")
}
func (c *setCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	var (
		r   io.Reader
		err error
	)
	if len(f.Args()) > 1 {
		return subcommands.ExitUsageError
	} else if len(f.Args()) == 1 {
		r, err = os.Open(f.Args()[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "set: while opening", f.Args()[0], ":", err)
			return subcommands.ExitFailure
		}
	} else {
		r = c.cfg.input
	}
	input, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, "set: while reading input:", err)
		return subcommands.ExitFailure
	}
	var (
		stage note.Stage
		n     note.Plain
	)
	err = yaml.UnmarshalNote(input, &n)
	if err != nil {
		fmt.Fprintln(os.Stderr, "set: while parsing input", err)
		return subcommands.ExitFailure
	}
	if n.ID == note.EmptyID {
		fmt.Fprintln(os.Stderr, "set: a non-zero id is required")
		return subcommands.ExitFailure
	}
	if err = stageAddPlain(&stage, &n); err != nil {
		fmt.Fprintln(os.Stderr, "set: while expanding input:", err)
		return subcommands.ExitFailure
	}

	db, err := c.cfg.open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "set: while opening db:", err)
		return subcommands.ExitFailure
	}
	defer db.Close()

	if err = db.IsolatedWrite(func(w note.FindLoadPatcher) error {
		return w.Patch(stage.Ops)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "set: while applying change:", err)
		return subcommands.ExitFailure
	}

	fmt.Fprintln(c.cfg.output, n.ID)
	return subcommands.ExitSuccess
}

// stageAddPlain expands stage to include the note described by p and all of
// its contents.
//
// The note maps storage backends represent notes with a value, an optional
// value type, and a sequence of contents, so only those aspects of p are
// turned into operations. Content notes without an ID cannot be stored and are
// skipped.
func stageAddPlain(stage *note.Stage, p *note.Plain) error {
	stn := stage.Note(p.ID)
	if p.ValueString != "" || p.ValueType != nil {
		var vt note.ID
		if p.ValueType != nil {
			vt = p.ValueType.ID
		}
		if err := stn.SetValue(p.ValueString, vt); err != nil {
			return err
		}
	}
	for _, c := range p.Contents {
		if c.ID == note.EmptyID {
			continue
		}
		if err := stageAddPlain(stage, c); err != nil {
			return err
		}
		if _, err := stn.AddContent(c.ID); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	subcommands.Register(&setCmd{&globalConfig}, "note")
}
