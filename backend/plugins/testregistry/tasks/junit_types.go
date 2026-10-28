/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"encoding/xml"
	"regexp"
	"sync"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
)

// DefaultJUnitRegexPattern is the default regex pattern for matching JUnit XML file names.
// Matches files starting with "devlake-", "e2e", or "qd-report-" and ending with .xml or .junit.
// Users can override this per-connection via the junitRegex field.
const DefaultJUnitRegexPattern = `(devlake-|e2e|qd-report-)[0-9a-z-]+\.(xml|junit)`

// JUnitRegexpSearch is the compiled default regex for backwards compatibility
// Deprecated: Use GetJUnitRegex(pattern) instead for configurable regex support
var JUnitRegexpSearch = regexp.MustCompile(DefaultJUnitRegexPattern)

// maxRegexCacheSize limits the number of cached regex patterns to prevent unbounded memory growth
const maxRegexCacheSize = 100

// regexCache caches compiled regex patterns to avoid recompilation
var regexCache = struct {
	sync.RWMutex
	patterns map[string]*regexp.Regexp
}{
	patterns: make(map[string]*regexp.Regexp),
}

// GetJUnitRegex returns a compiled regex for the given pattern, or the default if pattern is empty.
// The compiled regex is cached for performance.
//
// Parameters:
//   - pattern: The regex pattern string to compile. If empty, DefaultJUnitRegexPattern is used.
//   - logger: Optional logger for error reporting (can be nil)
//
// Returns:
//   - *regexp.Regexp: The compiled regex pattern
//   - errors.Error: Any error if the pattern is invalid
func GetJUnitRegex(pattern string, logger log.Logger) (*regexp.Regexp, errors.Error) {
	// Use default pattern if not specified
	if pattern == "" {
		return JUnitRegexpSearch, nil
	}

	// Check cache first
	regexCache.RLock()
	if cached, ok := regexCache.patterns[pattern]; ok {
		regexCache.RUnlock()
		return cached, nil
	}
	regexCache.RUnlock()

	// Compile the pattern
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		if logger != nil {
			logger.Warn(err, "invalid JUnit regex pattern, falling back to default", "pattern", pattern)
		}
		// Return default regex on compilation error for backwards compatibility
		return JUnitRegexpSearch, errors.BadInput.Wrap(err, "invalid JUnit regex pattern")
	}

	// Cache the compiled pattern (with size limit)
	regexCache.Lock()
	if len(regexCache.patterns) < maxRegexCacheSize {
		regexCache.patterns[pattern] = compiled
	}
	regexCache.Unlock()

	return compiled, nil
}

// GetJUnitRegexOrDefault returns the compiled regex for the given pattern, falling back to default on error.
// This is a convenience function that never returns an error.
//
// Parameters:
//   - pattern: The regex pattern string to compile. If empty, DefaultJUnitRegexPattern is used.
//   - logger: Optional logger for error reporting (can be nil)
//
// Returns:
//   - *regexp.Regexp: The compiled regex pattern (always returns a valid regex)
func GetJUnitRegexOrDefault(pattern string, logger log.Logger) *regexp.Regexp {
	regex, err := GetJUnitRegex(pattern, logger)
	if err != nil {
		return JUnitRegexpSearch
	}
	return regex
}

// The below types are directly marshalled into XML. The types correspond to jUnit
// XML schema, but do not contain all valid fields. For instance, the class name
// field for test cases is omitted, as this concept does not directly apply to Go.
// For XML specifications see http://help.catchsoftware.com/display/ET/JUnit+Format
// or view the XSD included in this package as 'junit.xsd'
//

// TestSuites represents a flat collection of jUnit test suites.
type TestSuites struct {
	XMLName xml.Name `xml:"testsuites"`

	// Suites are the jUnit test suites held in this collection
	Suites []*TestSuite `xml:"testsuite"`
}

// TestSuite represents a single jUnit test suite, potentially holding child suites.
type TestSuite struct {
	XMLName xml.Name `xml:"testsuite"`

	// Name is the name of the test suite
	Name string `xml:"name,attr"`

	// NumTests records the number of tests in the TestSuite
	NumTests uint `xml:"tests,attr"`

	// NumSkipped records the number of skipped tests in the suite
	NumSkipped uint `xml:"skipped,attr"`

	// NumFailed records the number of failed tests in the suite
	NumFailed uint `xml:"failures,attr"`

	// Duration is the time taken in seconds to run all tests in the suite
	Duration float64 `xml:"time,attr"`

	// Properties holds other properties of the test suite as a mapping of name to value
	Properties []*TestSuiteProperty `xml:"properties,omitempty"`

	// TestCases are the test cases contained in the test suite
	TestCases []*TestCase `xml:"testcase"`

	// Children holds nested test suites
	Children []*TestSuite `xml:"testsuite"` //nolint
}

// TestSuiteProperty contains a mapping of a property name to a value
type TestSuiteProperty struct {
	XMLName xml.Name `xml:"properties"`

	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// TestCase represents a jUnit test case
type TestCase struct {
	XMLName xml.Name `xml:"testcase"`

	// Name is the name of the test case
	Name string `xml:"name,attr"`

	// Classname is an attribute set by the package type and is required
	Classname string `xml:"classname,attr,omitempty"`

	// Duration is the time taken in seconds to run the test
	Duration float64 `xml:"time,attr"`

	// SkipMessage holds the reason why the test was skipped
	SkipMessage *SkipMessage `xml:"skipped"`

	// FailureOutput holds the output from a failing test
	FailureOutput *FailureOutput `xml:"failure"`

	// SystemOut is output written to stdout during the execution of this test case
	SystemOut string `xml:"system-out,omitempty"`

	// SystemErr is output written to stderr during the execution of this test case
	SystemErr string `xml:"system-err,omitempty"`
}

// SkipMessage holds a message explaining why a test was skipped
type SkipMessage struct {
	XMLName xml.Name `xml:"skipped"`

	// Message explains why the test was skipped
	Message string `xml:"message,attr,omitempty"`
}

// FailureOutput holds the output from a failing test
type FailureOutput struct {
	XMLName xml.Name `xml:"failure"`

	// Message holds the failure message from the test
	Message string `xml:"message,attr"`

	// Output holds verbose failure output from the test
	Output string `xml:",chardata"`
}
