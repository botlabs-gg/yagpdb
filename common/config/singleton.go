package config

var singleton = NewConfigManager()

func AddSource(source ConfigSource) {
	singleton.AddSource(source)
}

func RegisterOption(name, desc string, defaultValue interface{}) *ConfigOption {
	return singleton.RegisterOption(name, desc, defaultValue)
}

func Load() {
	singleton.Load()
}
