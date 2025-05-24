package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

type testContext struct {
	lastOutput   string
	lastError    error
	configPath   string
	createdFiles []string
}

func (c *testContext) aConfigurationFileWith(path string, content *godog.DocString) error {
	c.configPath = path
	c.createdFiles = append(c.createdFiles, path)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content.Content), 0644)
}

func (c *testContext) aFileWith(path string, content *godog.DocString) error {
	c.createdFiles = append(c.createdFiles, path)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(content.Content), 0644)
}

func (c *testContext) iRun(command string) error {
	args := strings.Fields(command)
	if args[0] != "go-batect" {
		return fmt.Errorf("unexpected command: %s", args[0])
	}

	// Build the binary if it doesn't exist or just use go run
	// For tests, let's use go run main.go
	runArgs := []string{"run", "main.go"}
	if c.configPath != "" {
		runArgs = append(runArgs, "-f", c.configPath)
	}
	runArgs = append(runArgs, args[1:]...)
	cmd := exec.Command("go", runArgs...)
	
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	
	c.lastError = cmd.Run()
	c.lastOutput = out.String()
	
	// Clean up files
	for _, f := range c.createdFiles {
		_ = os.Remove(f)
	}
	c.createdFiles = nil
	c.configPath = ""
	
	return nil
}
func (c *testContext) theOutputShouldContain(expected string) error {
	if !strings.Contains(c.lastOutput, expected) {
		return fmt.Errorf("expected output to contain %q, but got:\n%s", expected, c.lastOutput)
	}
	return nil
}

func InitializeScenario(sc *godog.ScenarioContext) {
	ctx := &testContext{}

	sc.Step(`^a configuration file "([^"].*)" with:$`, ctx.aConfigurationFileWith)
	sc.Step(`^a file "([^"].*)" with:$`, ctx.aFileWith)
	sc.Step(`^I run "([^"].*)"$`, ctx.iRun)
	sc.Step(`^the output should contain "([^"].*)"$`, ctx.theOutputShouldContain)
}

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
