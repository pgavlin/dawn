package dawn

import (
	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
)

// A Flag holds information about a project configuration flag.
type Flag struct {
	// Name holds the flag's name.
	Name string `json:"name"`
	// Default holds the string representation of the flag's default value.
	Default string `json:"default,omitempty"`
	// FlagType holds the string representation of the flag's type.
	FlagType string `json:"type"`
	// Choices holds string representations of the flag's vlaid values.
	Choices []string `json:"choices,omitempty"`
	// Required is true if the flag is required.
	Required bool `json:"required,omitempty"`
	// Help holds the flag's help message.
	Help string `json:"help,omitempty"`
	// Value holds the flag's value.
	Value starlark.Value `json:"-"`
}

func (f *Flag) Doc() string           { return f.Help }
func (f *Flag) String() string        { return "--" + f.Name }
func (f *Flag) Type() string          { return "flag" }
func (f *Flag) Freeze()               {} // immutable
func (f *Flag) Truth() starlark.Bool  { return starlark.True }
func (f *Flag) Hash() (uint32, error) { return starlark.String(f.Name).Hash() }

func (f *Flag) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(f.Name), nil
	case "default":
		return starlark.String(f.Default), nil
	case "type":
		return starlark.String(f.FlagType), nil
	case "choices":
		return util.StringList(f.Choices).List(), nil
	case "required":
		return starlark.Bool(f.Required), nil
	case "help", "__doc__":
		return starlark.String(f.Help), nil
	case "value":
		return f.Value, nil
	default:
		return nil, nil
	}
}

func (f *Flag) AttrNames() []string {
	return []string{"name", "default", "type", "choices", "required", "help", "__doc", "value"}
}
