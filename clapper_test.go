package clapper

import (
	"fmt"
	"testing"
	"time"
)

// This entire suite would be far easier with testify

var tests []struct {
	subCommand string
	arg        string
	longName   string
	shortName  string
	defaultVal interface{}
}

func setup(t *testing.T, args []string) (*CommandConfig, error) {
	tests = []struct {
		subCommand string
		arg        string
		longName   string
		shortName  string
		defaultVal interface{}
	}{
		{"", "output", "", "", ""},
		{"", "", "force", "f", false},
		{"", "", "verbose", "v", false},
		{"", "", "version", "V", ""},
		{"", "", "dir", "", "/var/users"},
		{"info", "category", "", "", []string{"manager", "student"}},
		{"info", "username", "", "", ""},
		{"info", "subjects...", "", "", ""},
		{"info", "", "verbose", "v", false},
		{"info", "", "version", "V", "1.0.1"},
		{"info", "", "output", "o", "./"},
		{"info", "", "no-clean", "", true},
		{"ghost", "", "", "", ""},
	}

	reg := NewRegistry()
	subs := make(map[string]*CommandConfig)
	for _, test := range tests {
		sub := subs[test.subCommand]
		if sub == nil {
			sub, _ = reg.Register(test.subCommand)
			subs[test.subCommand] = sub
		}
		if test.longName != "" {
			sub.AddFlag(test.longName, test.shortName, test.defaultVal)
		} else if test.arg != "" {
			sub.AddArg(test.arg, test.defaultVal)
		}
	}

	return reg.Parse(args)
}

/*----------------*/

// test unsupported flag
func TestUnsupportedAssignment(t *testing.T) {

	tests := []struct {
		flag     string
		expected string
		options  string
		err      error
	}{
		{"---version", "---version", "---version", UnknownFlag{}},
		{"---v", "---v", "---v=1.0.0", UnknownFlag{}},
		// Single-dash for long names was never supported, but now this is interpreted as:
		// -v -e -r -s -i -o -n; ergo, the error will be "unknown option '-e'"
		{"-e", "-e", "-version", UnknownFlag{}},
	}

	for _, test := range tests {
		reg := NewRegistry()
		root, _ := reg.Register("")
		root.AddFlag("version", "v", true)

		_, err := reg.Parse([]string{test.options})
		assertError(t, err, "(%s) %T: %v", test.options, test.err)
		if err != nil {
			assertEqual(t, fmt.Sprintf("%T", test.err), fmt.Sprintf("%T", err), "%s: %v", test.flag, err)
			var name string
			switch e := err.(type) {
			case UnknownFlag:
				name = e.Name
			default:
				assertError(t, err)
			}
			assertEqual(t, test.expected, name)
		}
	}
}

// test empty root command
func TestEmptyRootCommand(t *testing.T) {
	cmd, err := setup(t, []string{})

	assertNoError(t, err)
	assertEqual(t, "", cmd.Name)
	assertEqual(t, 1, len(cmd.Args))

	if cmd.Args["output"] == nil {
		for k := range cmd.Args {
			t.Errorf("expected one \"output\" argument; got %s", k)
		}
	}

}

func TestRootDefaults(t *testing.T) {
	cmd, err := setup(t, []string{})

	assertNoError(t, err)
	assertEqual(t, "", cmd.Args["output"].defaultValue)

	for _, test := range tests {
		if test.longName != "" && test.subCommand == "" {
			f := cmd.Flags[test.longName]
			assertNotNil(t, f, "missing flag %q", test.longName)
			assertEqual(t, test.shortName, f.ShortName, "(%s)", test.longName)
			dv := test.defaultVal
			assertEqual(t, dv, f.defaultValue, "(%+v %+v)", test, f)
		}
	}
}

// test an unregistered flag
func TestUnregisteredFlag(t *testing.T) {
	// flags
	flags := map[string][]string{
		"-d":          {"-V", "1.0.1", "-v", "--force", "-d", "./sub/dir"},
		"--m":         {"-V", "1.0.1", "-v", "--force", "--m", "./sub/dir"},
		"--directory": {"-V", "1.0.1", "-v", "--force", "--directory", "./sub/dir"},
	}

	for flag, options := range flags {
		_, err := setup(t, options)
		assertError(t, err)
		if e, ok := err.(UnknownFlag); !ok {
			t.Errorf("expected an UnknownFlag; got %T: %v", err, err)
		} else {
			assertEqual(t, flag, e.Name)
		}
	}
}

// test for valid inverted flag values
func TestValidInvertFlagValues(t *testing.T) {
	// options list
	optionsList := [][]string{
		{"info", "student", "-v", "--output", "./opt/dir", "--no-clean"},
		{"info", "student", "--no-clean", "--output", "./opt/dir", "--verbose"},
	}
	expecteds := map[string]interface{}{
		"clean":   false,
		"output":  "./opt/dir",
		"verbose": true,
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, options[0], cmd.Name)
		assertEqual(t, options[1], cmd.Args["category"].value, " (category)")
		assertEqual(t, nil, cmd.Args["username"].value, " (username)")
		assertEqual(t, nil, cmd.Args["subjects"].value, " (subjects)")
		for k, v := range expecteds {
			assertEqual(t, v, cmd.Flags[k].value, " (%s) %+v", k, cmd.Flags[k])
		}
	}
}

// test `--flag=value` syntax
func TestFlagAssignmentSyntax(t *testing.T) {
	// options list
	optionsList := [][]string{
		{"info", "student", "-v", "--version=2.0.0", "thatisuday"},
		{"info", "student", "thatisuday", "-v", "-V=2.0.0"},
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, "info", cmd.Name)
		assertEqual(t, "student", cmd.Args["category"].value)
		assertEqual(t, "thatisuday", cmd.Args["username"].value)
		assertEqual(t, nil, cmd.Args["subjects"].value)
		assertEqual(t, "2.0.0", cmd.Flags["version"].value)
		assertEqual(t, nil, cmd.Flags["output"].value)
		assertEqual(t, true, cmd.Flags["verbose"].value)
	}
}

// test for valid variadic argument values
func TestValidVariadicArgumentValues(t *testing.T) {

	// options list
	optionsList := [][]string{
		{"info", "student", "thatisuday", "-v", "--output", "./opt/dir", "--no-clean", "math", "science", "physics"},
		{"info", "student", "--no-clean", "thatisuday", "--output", "./opt/dir", "math", "science", "--verbose", "physics"},
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, "info", cmd.Name)
		assertEqual(t, "student", cmd.Args["category"].value)
		assertEqual(t, "thatisuday", cmd.Args["username"].value)
		assertEqual(t, []string{"math", "science", "physics"}, cmd.Args["subjects"].value)
		assertEqual(t, "./opt/dir", cmd.Flags["output"].value)
		assertEqual(t, true, cmd.Flags["verbose"].value)
		assertEqual(t, false, cmd.Flags["clean"].value)
	}
}

/*-------------------*/

// test root command with options
func TestRootCommandWithOptions(t *testing.T) {

	// options list
	optionsList := [][]string{
		{"userinfo", "-V", "1.0.1", "-v", "--force", "--dir", "./sub/dir"},
		{"-V", "1.0.1", "--verbose", "--force", "userinfo", "--dir", "./sub/dir"},
		{"-V", "1.0.1", "-v", "--force", "--dir", "./sub/dir", "userinfo"},
		{"--version", "1.0.1", "--verbose", "--force", "--dir", "./sub/dir", "userinfo"},
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, "", cmd.Name)
		assertEqual(t, "userinfo", cmd.Args["output"].value)
		assertEqual(t, true, cmd.Flags["force"].value)
		assertEqual(t, true, cmd.Flags["verbose"].value)
		assertEqual(t, "1.0.1", cmd.Flags["version"].value)
		assertEqual(t, "./sub/dir", cmd.Flags["dir"].value)
	}
}

// test sub-command with options
func TestSubCommandWithOptions(t *testing.T) {
	// options list
	optionsList := [][]string{
		{"info", "student", "-v", "--output", "./opt/dir"},
		{"info", "student", "--output", "./opt/dir", "--verbose"},
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, "info", cmd.Name)
		assertEqual(t, "student", cmd.Args["category"].value)
		assertEqual(t, nil, cmd.Args["username"].value)
		assertEqual(t, nil, cmd.Args["subjects"].value)
		assertEqual(t, "./opt/dir", cmd.Flags["output"].value)
		assertEqual(t, true, cmd.Flags["verbose"].value)
		assertEqual(t, nil, cmd.Flags["clean"].value)
	}
}

// test sub-command with valid and extra arguments
func TestSubCommandWithArguments(t *testing.T) {
	// options list
	optionsList := [][]string{
		{"info", "-v", "student", "-V", "2.0.0", "thatisuday"},
		{"info", "student", "-v", "thatisuday", "--version", "2.0.0"},
	}

	for _, options := range optionsList {
		cmd, err := setup(t, options)
		assertNoError(t, err)
		assertEqual(t, "info", cmd.Name)
		assertEqual(t, "student", cmd.Args["category"].value)
		assertEqual(t, "thatisuday", cmd.Args["username"].value)
		assertEqual(t, nil, cmd.Args["subjects"].value)
		assertEqual(t, "2.0.0", cmd.Flags["version"].value)
		assertEqual(t, nil, cmd.Flags["output"].value)
		assertEqual(t, true, cmd.Flags["verbose"].value)
	}
}

func TestValidate(t *testing.T) {
	var timeA, timeB time.Time
	var err error
	timeA, err = time.Parse("2006-01-02", "1985-08-01")
	assertNoError(t, err)
	timeB, err = time.Parse("2006-01-02", "1997-04-10")
	assertNoError(t, err)

	as := []struct {
		a Arg
		e bool
	}{
		// single values (type check)
		{a: Arg{Name: "int", value: 1, defaultValue: 0}, e: false},
		{a: Arg{Name: "string", value: "a", defaultValue: ""}, e: false},
		{a: Arg{Name: "bool", value: true, defaultValue: false}, e: false},
		{a: Arg{Name: "float", value: 1.0, defaultValue: 0.0}, e: false},
		{a: Arg{Name: "time", value: time.Now(), defaultValue: time.Now()}, e: false},
		{a: Arg{Name: "duration", value: time.Second, defaultValue: time.Minute}, e: false},
		// choices
		{a: Arg{Name: "int choice", value: 1, defaultValue: []int{1, 2, 3}}, e: false},
		{a: Arg{Name: "string choice", value: "a", defaultValue: []string{"b", "a"}}, e: false},
		{a: Arg{Name: "float choice", value: 1.0, defaultValue: []float64{0.0, 1.0}}, e: false},
		{a: Arg{Name: "time choice", value: timeA, defaultValue: []time.Time{timeA, timeB}}, e: false},
		{a: Arg{Name: "duration choice", value: 2 * time.Second, defaultValue: []time.Duration{2 * time.Second, time.Minute}}, e: false},
		// Failures
		{a: Arg{Name: "int choice fail", value: 1, defaultValue: []int{2, 3}}, e: true},
		{a: Arg{Name: "string choice fail", value: "a", defaultValue: []string{"b"}}, e: true},
		{a: Arg{Name: "float choice fail", value: 1.0, defaultValue: []float64{2.0}}, e: true},
		{a: Arg{Name: "time choice fail", value: timeA, defaultValue: []time.Time{timeB}}, e: true},
		{a: Arg{Name: "duration choice fail", value: time.Second, defaultValue: []time.Duration{time.Minute}}, e: true},
		// Variadics (type check)
		{a: Arg{Name: "int variadic", value: []int{1, 2}, defaultValue: 0}, e: false},
		{a: Arg{Name: "string variadic", value: []string{"a", "b"}, defaultValue: ""}, e: false},
		{a: Arg{Name: "float variadic", value: []float64{1.0, 2.0}, defaultValue: 0.0}, e: false},
		{a: Arg{Name: "time variadic", value: []time.Time{timeA, timeB}, defaultValue: time.Now()}, e: false},
		{a: Arg{Name: "duration variadic", value: []time.Duration{time.Second, 2 * time.Second}, defaultValue: time.Minute}, e: false},
		// Variadic choices
		{a: Arg{Name: "int variadic choice", value: []int{1, 2}, defaultValue: []int{1, 2, 3}}, e: false},
		{a: Arg{Name: "string variadic choice", value: []string{"a", "b"}, defaultValue: []string{"b", "a", "c"}}, e: false},
		{a: Arg{Name: "float variadic choice", value: []float64{1.0, 2.0}, defaultValue: []float64{0.0, 1.0, 2.0}}, e: false},
		{a: Arg{Name: "time variadic choice", value: []time.Time{timeA, timeB}, defaultValue: []time.Time{timeA, timeB, time.Now()}}, e: false},
		{a: Arg{Name: "duration variadic choice", value: []time.Duration{time.Second, 2 * time.Second}, defaultValue: []time.Duration{2 * time.Second, time.Minute, time.Second}}, e: false},
	}
	for _, a := range as {
		err := validateParams(&a.a)
		if a.e {
			assertError(t, err)
		} else {
			assertNoError(t, err)
		}
	}
}

// test sub-command with valid and extra arguments
func TestCombinedFlags(t *testing.T) {

	// tests
	tests := []struct {
		Name           string
		Args           []string
		ExpectedNames  []string
		ExpectedValues []string
	}{
		{"one flag", []string{"-a"}, []string{"alpha"}, []string{"true"}},
		{"two flags", []string{"-ab"}, []string{"alpha", "bravo"}, []string{"true", "true"}},
		{"two flags & var", []string{"-abc", "value"}, []string{"alpha", "bravo", "charlie"}, []string{"true", "true", "value"}},
	}

	for _, test := range tests {
		reg := NewRegistry()
		root, _ := reg.Register("")
		root.AddFlag("alpha", "a", true)
		root.AddFlag("bravo", "b", true)
		root.AddFlag("charlie", "c", "none")

		cmd, err := reg.Parse(test.Args)
		if err != nil {
			fmt.Printf("parse error %s\n", err)
			if len(test.ExpectedNames) > 0 {
				t.Errorf("(%s) unexpected parse error: %s", test.Name, err)
			}
			// else, expected error
			continue
		}
		if len(test.ExpectedNames) == 0 && err == nil {
			t.Errorf("(%s) expected parse error; didn't get one", test.Name)
		}
		// Check that all expected arguments are there
		for i, n := range test.ExpectedNames {
			var found bool
			for _, a := range cmd.Flags {
				if a.Name == n {
					if fmt.Sprintf("%v", a.value) != test.ExpectedValues[i] {
						t.Errorf("(%s) expected value %s for %s, got %s", test.Name, test.ExpectedValues[i], n, a.value)
					}
					found = true
				}
			}
			if !found {
				t.Errorf("(%s) did not find expected argument %s", test.Name, n)
			}
		}
	}
}
