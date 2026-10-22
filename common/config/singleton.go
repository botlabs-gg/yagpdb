package config

var Singleton = NewConfigManager()

func AddSource(source ConfigSource) {
	Singleton.AddSource(source)
}

func RegisterOption(name, desc string, defaultValue any) *ConfigOption {
	return Singleton.RegisterOption(name, desc, defaultValue)
}

func Load() {
	Singleton.Load()
}
