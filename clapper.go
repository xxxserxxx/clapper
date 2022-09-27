// MIT License

// Copyright (c) 2020 Uday Hiwarale

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package clapper processes the command-line arguments of getopt(3) syntax.
// This package provides the ability to process the root command, sub commands,
// command-line arguments and command-line flags.
package clapper

// TODO descriptions for help

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

/***********************************************/

// UnknownCommand represents an error when command-line arguments contain an unregistered command.
type UnknownCommand struct {
	Name string
}

func (e UnknownCommand) Error() string {
	return fmt.Sprintf("unknown command %s found in the arguments", e.Name)
}

type BadArgument struct {
	Arg     *Arg
	Message string
}

func (e BadArgument) Error() string {
	return fmt.Sprintf("%s %s", e.Arg.Name, e.Message)
}

// UnknownFlag represents an error when command-line arguments contain an unregistered flag.
type UnknownFlag struct {
	Name string
}

func (e UnknownFlag) Error() string {
	return fmt.Sprintf("unknown flag %s found in the arguments", e.Name)
}

/*---------------------*/

// Registry holds the configuration of the registered commands.
type Registry map[string]*CommandConfig

// NewRegistry returns new instance of the "Registry"
func NewRegistry() Registry {
	return make(Registry)
}

// Register method registers a command.
// The "name" argument should be a simple string.
// If "name" is an empty string, it is considered as a root command.
// If a command is already registered, the registered `*CommandConfig` object is returned.
// If the command is already registered, second return value will be `true`.
func (registry Registry) Register(name string) (*CommandConfig, bool) {

	// remove all whitespaces
	commandName := removeWhitespaces(name)

	// check if command is already registered, if found, return existing entry
	if _commandConfig, ok := registry[commandName]; ok {
		return _commandConfig, true
	}

	// construct new `CommandConfig` object
	commandConfig := &CommandConfig{
		Name:       commandName,
		Flags:      make(map[string]*Flag),
		flagsShort: make(map[string]string),
		Args:       make(map[string]*Arg),
		ArgNames:   make([]string, 0),
	}

	// add entry to the registry
	registry[commandName] = commandConfig

	return commandConfig, false
}

// Parse method parses command-line arguments and returns an appropriate "*CommandConfig" object registered in the registry.
// If command is not registered, it return `ErrorUnknownCommand` error.
// If there is an error parsing a flag, it can return an `ErrorUnknownFlag` or `ErrorUnsupportedFlag` error.
func (registry Registry) Parse(values []string) (*CommandConfig, error) {

	// command name
	var commandName string

	// command-line argument values to process
	valuesToProcess := values

	// check if command is a root command
	if isRootCommand(values, registry) {
		commandName = "" // root command name
	} else {
		commandName, valuesToProcess = nextValue(values)
	}

	// format command-line argument values
	valuesToProcess = formatCommandValues(valuesToProcess)

	// check for invalid flag structure
	for _, val := range valuesToProcess {
		if isFlag(val) && isUnknownFlag(val) {
			return nil, UnknownFlag{val}
		}
	}

	// if command is not registered, return `ErrorUnknownCommand` error
	if _, ok := registry[commandName]; !ok {
		return nil, UnknownCommand{commandName}
	}

	// get `CommandConfig` object from the registry
	commandConfig := registry[commandName]

	// process all command-line arguments (except command name)
	for {

		// get current command-line argument value
		var value string
		value, valuesToProcess = nextValue(valuesToProcess)

		// if `value` is empty, break the loop
		if len(value) == 0 {
			break
		}

		// if the thing is a flag, process as a flag; otherwise, process as an arg
		if isFlag(value) {

			// trim `-` characters from the `value`
			name := strings.TrimLeft(value, "-")

			// get flag object stored in the `commandConfig`
			var flag *Flag
			var isBool bool

			// check if flag is short or long
			if isShortFlag(value) {
				if _, ok := commandConfig.flagsShort[name]; !ok {
					return nil, UnknownFlag{value}
				}

				// get long flag name
				flagName := commandConfig.flagsShort[name]

				flag = commandConfig.Flags[flagName]

				_, isBool = flag.defaultValue.(bool)

				// there is no argument; just set the value
				if isBool {
					flag.value = true
				}

			} else {

				// If it's a long flag, the check for bool (which has no value)
				var isInv bool
				if strings.HasPrefix(name, "no-") {
					name = name[3:]
					isInv = true
				}
				flag = commandConfig.Flags[name]
				if flag == nil {
					return nil, UnknownFlag{value}
				}
				_, isBool = flag.defaultValue.(bool)
				if isBool {
					flag.value = !isInv
				}
				if isInv {
					if !isBool {
						return nil, BadArgument{&flag.Arg, "non-bool flag"}
					}
				}
			}
			var err error
			if !isBool {
				if nextValue, nextValuesToProcess := nextValue(valuesToProcess); len(nextValue) != 0 && !isFlag(nextValue) {
					if flag.value, err = convert(nextValue, flag.defaultValue); err != nil {
						return nil, err
					}
					valuesToProcess = nextValuesToProcess
				} else if len(nextValue) == 0 {
					return nil, BadArgument{&flag.Arg, "parameter requires an argument, none was provided"}
				}
			}
			if err := validateParams(&flag.Arg); err != nil {
				return nil, err
			}
		} else {

			// process as argument
			var arg *Arg
			//var err error
			for index, argName := range commandConfig.ArgNames {
				// get argument object stored in the `commandConfig`
				arg = commandConfig.Args[argName]

				var conval interface{}
				var err error
				if conval, err = convert(value, arg.defaultValue); err != nil {
					return nil, err
				}
				var slice reflect.Value
				if arg.value == nil {
					if arg.isVariadic {
						slice = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(conval)), 0, 0)
						rval := reflect.New(slice.Type())
						rval.Elem().Set(slice)
						sp := reflect.ValueOf(rval.Interface())
						svp := sp.Elem()
						arg.value = svp.Interface()
					} else {
						arg.value = conval
						break
					}
				}

				// if last argument is a variadic argument, append values
				if (index == len(commandConfig.ArgNames)-1) && arg.isVariadic {
					slice = reflect.ValueOf(arg.value)
					rval := reflect.New(slice.Type())
					rval.Elem().Set(slice)
					sp := reflect.ValueOf(rval.Interface())
					svp := sp.Elem()
					svp.Set(reflect.Append(svp, reflect.ValueOf(conval)))
					arg.value = svp.Interface()
				}
			}
			if err := validateParams(arg); err != nil {
				return nil, err
			}
		}
	}

	return commandConfig, nil
}

func convert(i string, defaults interface{}) (interface{}, error) {
	var rv interface{}
	var err error
	// The default could be an array of allowed values, and if so,
	// get one of the elements so we can test the type
	p := reflect.TypeOf(defaults)
	if p.Kind() == reflect.Slice {
		p = p.Elem()
	}
	timeKind := reflect.TypeOf(time.Now()).Kind()
	durationKind := reflect.TypeOf(time.Second).Kind()
	switch p.Kind() {
	case reflect.Bool:
		rv, err = strconv.ParseBool(i)
	case reflect.String:
		return i, nil
	case reflect.Int:
		rv, err = strconv.Atoi(i)
	case timeKind:
		rv, err = time.Parse("2006-01-02 03:04", i)
	case durationKind:
		rv, err = time.ParseDuration(i)
	case reflect.Float64:
		rv, err = strconv.ParseFloat(i, 64)
	default:
	}
	return rv, err
}

// validate the a.value(s) against the a.defaultValue(s)
// If a.value is a value, it must match the type of a.defaultValue; or,
// if a.defaultValue is an array, a.value must be in a.defaultValue.
//
// If a.value is an array, every element must be of type a.defaultValue; or,
// if a.defaultValue is an array, every element in a.value mut be found in a.defaultValue.
func validateParams(a *Arg) error {
	if a.value == nil {
		return BadArgument{a, "parameter requires argument"}
	}
	p := reflect.TypeOf(a.value)
	pv := reflect.ValueOf(a.value)
	// if a.value is an array, check each element against a.defaultValues
	if p.Kind() == reflect.Slice {
		for i := 0; i < pv.Len(); i++ {
			v := pv.Index(i).Interface()
			if !validateElement(v, a.defaultValue) {
				return BadArgument{a, fmt.Sprintf("illegal value %v, must be %v", v, a.defaultValue)}
			}
		}
		return nil
	} else {
		// if a.value is not an array, test it against a.defaultValue
		if !validateElement(a.value, a.defaultValue) {
			return BadArgument{a, fmt.Sprintf("illegal value %v, must be %v", a.value, a.defaultValue)}
		}
	}
	return nil
}

// validates a single argument against a (possible) array
func validateElement(val interface{}, vals interface{}) bool {
	p := reflect.TypeOf(vals)
	pv := reflect.ValueOf(vals)
	// if vals is an array, val must be in it
	if p.Kind() == reflect.Slice {
		for i := 0; i < pv.Len(); i++ {
			v := pv.Index(i).Interface()
			if val == v {
				return true
			}
		}
		return false
	} else {
		// Both val and vals are not arrays
		return reflect.TypeOf(val) == reflect.TypeOf(vals)
	}
}

/*---------------------*/

// CommandConfig type holds the structure and values of the command-line arguments of command.
type CommandConfig struct {

	// name of the sub-command ("" for the root command)
	Name string

	// command-line flags
	Flags map[string]*Flag

	// mapping of the short flag names with long flag names
	flagsShort map[string]string

	// registered command argument values
	Args map[string]*Arg

	// list of the argument names (for ordered iteration)
	ArgNames []string
}

// AddArg registers an argument configuration with the command.
//
//   - The `name` argument represents the name of the argument.
//   - If value of the `name` argument ends with `...` suffix, then it is a
//     variadic argument.
//   - If the argument is already registered, second return value will be `true`.
//   - Variadic argument can accept multiple argument values and it should be the
//     last registered argument.
//   - Values of a variadic argument will be returned as an array.
//   - All arguments without a default value must be registered first.
//   - If an argument with given `name` is already registered, then argument
//     registration is skipped and registered `*Arg` object returned.
//   - The `defaultValue` argument represents the default value of the argument,
//     and determines the type of the argument value.
//
// Supported types are:
//
//   - int
//   - string
//   - float64
//   - bool
//   - time.Time
//   - time.Duration
//
// If the provided defaultValue is an array of one of the above types, then that
// array defines a set of legal values; any provided parameter that doesn't
// match a value in the array will result in a parse error.
func (commandConfig *CommandConfig) AddArg(name string, defaultValue interface{}) *Arg {

	// clean argument values
	name = removeWhitespaces(name)

	// return if argument is already registered
	if _arg, ok := commandConfig.Args[name]; ok {
		return _arg
	}

	rv := Arg{Name: name}

	if v, ok := defaultValue.(string); ok {
		rv.defaultValue = trimWhitespaces(v)
	} else {
		rv.defaultValue = defaultValue
	}

	// check if argument is variadic
	if ok, argName := isVariadicArgument(name); ok {
		rv.Name = argName // change argument name
		rv.isVariadic = true
	}

	// register argument with the command-config
	commandConfig.Args[rv.Name] = &rv

	// store argument name (for ordered iteration)
	commandConfig.ArgNames = append(commandConfig.ArgNames, rv.Name)

	return &rv
}

// AddFlag method registers a command-line flag with the command.
//
// The rules for flags are the same as for args, but in addition:
//
//   - The `name` argument is the long-name of the flag and it should not start
//     with `--` prefix.
//   - The `shortName` argument is the short-name of the flag and it should not
//     start with `-` prefix.
//   - A boolean flag doesn't accept an input value such as `--flag=<value>` and
//     its default value is "true".
//   - If the `name` value starts with `no-` prefix, then it is considered as an
//     inverted flag.
//   - Regardless of the default, boolean flags are true if provided, and false if
//     inverted
//   - Registering a non-boolean inverted flag will produce an error
//   - Boolean flag defaults are preserved, but have no effect on the `AsBool()` result.
func (commandConfig *CommandConfig) AddFlag(name string, shortName string, defaultValue interface{}) (*Flag, error) {
	// clean argument values
	name = removeWhitespaces(name)

	isInverted := strings.HasPrefix(name, "no-")

	if isInverted {
		name = name[3:]
	}
	if _, ok := defaultValue.(bool); !ok && isInverted {
		return nil, fmt.Errorf("non-boolean arguments can not be inverted")
	}

	// return if flag is already registered
	if _flag, ok := commandConfig.Flags[name]; ok {
		return _flag, nil
	}

	rv := Flag{
		ShortName: removeWhitespaces(shortName),
	}
	rv.Name = name
	rv.defaultValue = defaultValue

	switch v := defaultValue.(type) {
	case bool:
		if isInverted {
			rv.defaultValue = !v
		} else {
			rv.defaultValue = v
		}
	case string:
		rv.defaultValue = trimWhitespaces(v)
	}

	// check if argument is variadic
	if ok, argName := isVariadicArgument(name); ok {
		rv.Name = argName // change argument name
		rv.isVariadic = true
	}

	// short flag name should be only one character long
	l := len(rv.ShortName)
	switch {
	case l > 1:
		return nil, fmt.Errorf("short names must be one character")
	case l == 1:
		if isInverted {
			return nil, fmt.Errorf("inverted flags may not have short versions")
		}
		rv.ShortName = rv.ShortName[:1]
		commandConfig.flagsShort[rv.ShortName] = rv.Name
	}

	// register flag with the command-config
	commandConfig.Flags[rv.Name] = &rv

	return &rv, nil
}

// Arg type holds the structured information about an argument.
type Arg struct {
	// name of the argument
	Name string

	isVariadic   bool
	defaultValue interface{}
	value        interface{}
}

func (a Arg) AsInt() int {
	if v, ok := a.value.(int); ok {
		return v
	} else {
		v, _ = a.defaultValue.(int)
		return v
	}
}

func (a Arg) AsTime() time.Time {
	if v, ok := a.value.(time.Time); ok {
		return v
	} else {
		v, _ := a.defaultValue.(time.Time)
		return v
	}
}

func (a Arg) AsDuration() time.Duration {
	if v, ok := a.value.(time.Duration); ok {
		return v
	} else {
		v, _ = a.defaultValue.(time.Duration)
		return v
	}
}

func (a Arg) AsBool() bool {
	if v, ok := a.value.(bool); ok {
		return v
	} else {
		v, _ = a.defaultValue.(bool)
		return v
	}
}

func (a Arg) AsString() string {
	if v, ok := a.value.(string); ok {
		return v
	} else {
		v, _ = a.defaultValue.(string)
		return v
	}
}

func (a Arg) AsFloat() float64 {
	if v, ok := a.value.(float64); ok {
		return v
	} else {
		v, _ = a.defaultValue.(float64)
		return v
	}
}

func (a Arg) AsInts() []int {
	if v, ok := a.value.([]int); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]int)
		return v
	}
}

func (a Arg) AsTimes() []time.Time {
	if v, ok := a.value.([]time.Time); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]time.Time)
		return v
	}
}

func (a Arg) AsDurations() []time.Duration {
	if v, ok := a.value.([]time.Duration); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]time.Duration)
		return v
	}
}

func (a Arg) AsBools() []bool {
	if v, ok := a.value.([]bool); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]bool)
		return v
	}
}

func (a Arg) AsStrings() []string {
	if v, ok := a.value.([]string); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]string)
		return v
	}
}

func (a Arg) AsFloats() []float64 {
	if v, ok := a.value.([]float64); ok {
		return v
	} else {
		v, _ = a.defaultValue.([]float64)
		return v
	}
}

// Flag type holds the structured information about a flag.
type Flag struct {
	Arg
	// short name of the flag
	ShortName string
}

/***********************************************
        PRIVATE FUNCTIONS AND VARIABLES
***********************************************/

// format command-line argument values
func formatCommandValues(values []string) (formatted []string) {

	formatted = make([]string, 0)

	for _, presplit := range values {
		for _, value := range detectSplitCombined(presplit) {
			// split a value by `=`
			if isFlag(value) {
				parts := strings.Split(value, "=")

				for _, part := range parts {
					if strings.Trim(part, " ") != "" {
						formatted = append(formatted, part)
					}
				}
			} else {
				formatted = append(formatted, value)
			}
		}
	}
	fmt.Printf("formatted = %v\n", formatted)

	return
}

// detectSplitCombined checks whether the argument is a combined flag and breaks
// it apart if it is. E.g., for declared flags `-a` and `-b`, the provided
// argument `-ab` will be broken in two.
func detectSplitCombined(s string) []string {
	// Don't try to process assignments
	if strings.Contains(s, "=") {
		return []string{s}
	}
	// If it's a long-form argument, then no split
	if strings.HasPrefix(s, "--") {
		return []string{s}
	}
	if strings.HasPrefix(s, "-") {
		parts := strings.Split(s, "")
		for i, p := range parts {
			if i == 0 {
				continue
			}
			parts[i] = "-" + p
		}
		return parts[1:]
	}
	return []string{s}
}

// check if value is a flag
func isFlag(value string) bool {
	return len(value) >= 2 && strings.HasPrefix(value, "-")
}

// check if value is a short flag
func isShortFlag(value string) bool {
	return isFlag(value) && len(value) == 2 && !strings.HasPrefix(value, "--")
}

// check if flag is unsupported
func isUnknownFlag(value string) bool {

	// a flag should be at least two characters log
	if len(value) >= 2 {

		// if short flag, it should start with `-` but not with `--`
		if len(value) == 2 {
			return !strings.HasPrefix(value, "-") || strings.HasPrefix(value, "--")
		}

		// if long flag, it should start with `--` and not with `---`
		return !strings.HasPrefix(value, "--") || strings.HasPrefix(value, "---")
	}

	return false
}

// check if value ends with `...` sufix
func isVariadicArgument(value string) (bool, string) {
	if !isFlag(value) && strings.HasSuffix(value, "...") {
		return true, strings.TrimRight(value, "...") // trim `...` suffix
	}

	return false, ""
}

// check if values corresponds to the root command
func isRootCommand(values []string, registry Registry) bool {

	// FALSE: if the root command is not registered
	if _, ok := registry[""]; !ok {
		return false
	}

	// TRUE: if all `values` are empty or the first `value` is a flag
	if len(values) == 0 || isFlag(values[0]) {
		return true
	}

	// get root `CommandConfig` value from the registry
	rootCommandConfig := registry[""]

	// TRUE: if the first value is not a registered command
	// and some arguments are registered for the root command
	if _, ok := registry[values[0]]; len(rootCommandConfig.Args) > 0 && !ok {
		return true
	}

	return false
}

// return next value and remaining values of a slice of strings
func nextValue(slice []string) (v string, newSlice []string) {

	if len(slice) == 0 {
		v, newSlice = "", make([]string, 0)
		return
	}

	v = slice[0]

	if len(slice) > 1 {
		newSlice = slice[1:]
	} else {
		newSlice = make([]string, 0)
	}

	return
}

// trim whitespaces from a value
func trimWhitespaces(value string) string {
	return strings.Trim(value, "")
}

// remove whitespaces from a value
func removeWhitespaces(value string) string {
	return strings.ReplaceAll(value, " ", "")
}
