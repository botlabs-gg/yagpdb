package config

import (
	"strconv"
	"strings"
)

type ConfigSource interface {
	GetValue(key string) interface{}
	Name() string
}

type ConfigOption struct {
	Name         string
	Description  string
	DefaultValue interface{}
	LoadedValue  interface{}
	Manager      *ConfigManager

	ConfigSource ConfigSource
}

func (opt *ConfigOption) LoadValue() {
	newVal := opt.DefaultValue
	opt.ConfigSource = nil

	for i := len(opt.Manager.sources) - 1; i >= 0; i-- {
		source := opt.Manager.sources[i]

		v := source.GetValue(opt.Name)
		if v != nil {
			newVal = v
			opt.ConfigSource = source
			break
		}
	}

	// parse ahead of time
	if opt.DefaultValue != nil {
		if _, ok := opt.DefaultValue.(int); ok {
			newVal = interface{}(intVal(newVal))
		} else if _, ok := opt.DefaultValue.(bool); ok {
			newVal = interface{}(boolVal(newVal))
		}
	}

	opt.LoadedValue = newVal
}

func (opt *ConfigOption) GetString() string {
	return strVal(opt.LoadedValue)
}

func (opt *ConfigOption) GetInt() int {
	return intVal(opt.LoadedValue)
}

func (opt *ConfigOption) GetBool() bool {
	return boolVal(opt.LoadedValue)
}

type ConfigManager struct {
	sources []ConfigSource
	Options map[string]*ConfigOption
}

func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		Options: make(map[string]*ConfigOption),
	}
}

func (c *ConfigManager) AddSource(source ConfigSource) {
	c.sources = append(c.sources, source)
}

func (c *ConfigManager) RegisterOption(name, desc string, defaultValue interface{}) *ConfigOption {
	opt := &ConfigOption{
		Name:         name,
		Description:  desc,
		DefaultValue: defaultValue,
		Manager:      c,
	}

	c.Options[name] = opt
	return opt
}

func (c *ConfigManager) Load() {
	for _, v := range c.Options {
		v.LoadValue()
	}
}

func strVal(i interface{}) string {
	switch t := i.(type) {
	case string:
		return t
	case int:
		return strconv.FormatInt(int64(t), 10)
	case Stringer:
		return t.String()
	}

	return ""
}

type Stringer interface {
	String() string
}

func intVal(i interface{}) int {
	switch t := i.(type) {
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return int(n)
	case int:
		return t
	}

	return 0
}

func boolVal(i interface{}) bool {
	switch t := i.(type) {
	case string:
		lower := strings.ToLower(strings.TrimSpace(t))
		if lower == "true" || lower == "yes" || lower == "on" || lower == "enabled" || lower == "1" {
			return true
		}

		return false
	case int:
		return t > 0
	case bool:
		return t
	}

	return false
}
