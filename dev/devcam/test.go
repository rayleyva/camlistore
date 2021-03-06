/*
Copyright 2013 The Camlistore Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file adds the "test" subcommand to devcam, to run the full test suite.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"camlistore.org/pkg/cmdmain"
)

type testCmd struct {
	// start of flag vars
	short bool
	// end of flag vars

	// buildGoPath becomes our child "go" processes' GOPATH environment variable
	buildGoPath string
}

func init() {
	cmdmain.RegisterCommand("test", func(flags *flag.FlagSet) cmdmain.CommandRunner {
		cmd := new(testCmd)
		flags.BoolVar(&cmd.short, "short", false, "Use '-short' with go test.")
		return cmd
	})
}

func (c *testCmd) Usage() {
	fmt.Fprintf(cmdmain.Stderr, "Usage: devcam test\n")
}

func (c *testCmd) Describe() string {
	return "run the full test suite."
}

func (c *testCmd) RunCommand(args []string) error {
	if len(args) != 0 {
		c.Usage()
	}
	if err := c.syncSrc(); err != nil {
		return err
	}
	buildSrcDir := filepath.Join(c.buildGoPath, "src", "camlistore.org")
	if err := os.Chdir(buildSrcDir); err != nil {
		return err
	}
	if err := c.buildSelf(); err != nil {
		return err
	}
	if err := c.genKeyBlob(); err != nil {
		return err
	}
	if err := c.runTests(); err != nil {
		return err
	}
	println("PASS")
	return nil
}

func (c *testCmd) env() *Env {
	if c.buildGoPath == "" {
		panic("called too early")
	}
	env := NewCopyEnv()
	env.NoGo()
	env.Set("GOPATH", c.buildGoPath)
	return env
}

func (c *testCmd) syncSrc() error {
	args := []string{"run", "make.go", "--onlysync"}
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error populating tmp src tree: %v", err)
	}
	c.buildGoPath = strings.TrimSpace(string(out))
	return nil
}

func (c *testCmd) buildSelf() error {
	args := []string{
		"install",
		filepath.FromSlash("./dev/devcam"),
	}
	cmd := exec.Command("go", args...)
	binDir, err := filepath.Abs("bin")
	if err != nil {
		return fmt.Errorf("Error setting GOBIN: %v", err)
	}
	env := c.env()
	env.Set("GOBIN", binDir)
	cmd.Env = env.Flat()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error building devcam: %v", err)
	}
	return nil
}

func (c *testCmd) genKeyBlob() error {
	cmdBin := filepath.FromSlash("./bin/devcam")
	args := []string{
		"put",
		"init",
		"--gpgkey=" + defaultKeyID,
		"--noconfig",
	}
	cmd := exec.Command(cmdBin, args...)
	env := c.env()
	env.Set("CAMLI_SECRET_RING", filepath.FromSlash(defaultSecring))
	cmd.Env = env.Flat()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error generating keyblobs: %v", err)
	}
	return nil
}

func (c *testCmd) runTests() error {
	args := []string{"test"}
	if !strings.HasSuffix(c.buildGoPath, "-nosqlite") {
		args = append(args, "--tags=with_sqlite")
	}
	if c.short {
		args = append(args, "-short")
	}
	args = append(args, []string{
		"./pkg/...",
		"./server/camlistored",
		"./server/appengine",
		"./cmd/...",
	}...)
	env := c.env()
	env.Set("SKIP_DEP_TESTS", "1")
	return runExec("go", args, env)
}
