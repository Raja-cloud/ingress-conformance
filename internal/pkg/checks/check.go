package checks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Config struct {
}

type Check struct {
	Name        string
	Description string

	Run func(check *Check, config Config) (bool, error)

	// All checks
	checks []*Check

	// Parent check
	parent *Check
}

type CapturedRequest struct {
	StatusCode int
	TestId     string
	Path       string
	Host       string
}

func captureRequest(location string, hostOverride string) (data CapturedRequest, err error) {
	client := &http.Client{
		Timeout: time.Second * 3,
	}
	req, err := http.NewRequest("GET", location, nil)
	if err != nil {
		return
	}
	if hostOverride != "" {
		req.Host = hostOverride
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return
	}

	data.StatusCode = resp.StatusCode
	return
}

type assertionSet []error
type assert struct {
	expect        interface{}
	actual        interface{}
	errorTemplate string
}

func (a *assertionSet) equals(assert assert) {
	if assert.expect != assert.actual {
		err := fmt.Errorf(assert.errorTemplate, assert.actual, assert.expect)
		*a = append(*a, err)
	}
}

func (a *assertionSet) Error() (err string) {
	for i, e := range *a {
		err += fmt.Sprintf("\t%d) Assertion failed: %s\n", i+1, e.Error())
	}
	return
}

func (c *Check) AddCheck(checks ...*Check) {
	for i, x := range checks {
		if checks[i] == c {
			panic("Checks can't be a child of itself")
		}
		checks[i].parent = c
		c.checks = append(c.checks, x)
	}
}

var Checks = &Check{
	Name: "all",
}

func (c Check) List() {
	if c.Description != "" {
		fmt.Printf("- %s (%s)\n", c.Description, c.Name)
	}
	for _, check := range c.checks {
		check.List()
	}
}

func (c Check) Verify(filterOnCheckName string, config Config) (successCount int, failureCount int, err error) {
	if filterOnCheckName != c.Name && filterOnCheckName != "" {
		for _, check := range c.checks {
			s, f, err := check.Verify(filterOnCheckName, config)
			successCount += s
			failureCount += f
			if err != nil {
				fmt.Printf(err.Error())
			}
		}

		return
	}

	fmt.Printf("Running '%s' verifications...\n", c.Name)
	runChildChecks := true
	if c.Run != nil {
		success, err := c.Run(&c, config)
		if err != nil {
			fmt.Printf("  %s\n", err.Error())
		}

		if success {
			successCount++
		} else {
			failureCount++
			runChildChecks = false
			fmt.Printf("  Check failed: %s\n", c.Name)
		}
	}

	if runChildChecks {
		for _, check := range c.checks {
			s, f, err := check.Verify("", config)
			if err != nil {
				fmt.Printf(err.Error())
			}
			successCount += s
			failureCount += f
		}
	}

	return
}
