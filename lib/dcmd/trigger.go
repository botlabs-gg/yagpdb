package dcmd

type Trigger struct {
	Names       []string
	Middlewares []MiddleWareFunc

	HideFromHelp bool

	EnableInDM            bool
	EnableInGuildChannels bool
	EnableInThreads       bool
}

func NewTrigger(name string, aliases ...string) *Trigger {
	names := []string{name}
	if len(aliases) > 0 {
		names = append(names, aliases...)
	}

	return &Trigger{
		Names:                 names,
		EnableInDM:            true,
		EnableInGuildChannels: true,
		EnableInThreads:       true,
	}
}

func (t *Trigger) SetHideFromHelp(hide bool) *Trigger {
	t.HideFromHelp = hide
	return t
}

func (t *Trigger) SetEnableInDM(enable bool) *Trigger {
	t.EnableInDM = enable
	return t
}

func (t *Trigger) SetEnableInGuildChannels(enable bool) *Trigger {
	t.EnableInGuildChannels = enable
	return t
}

func (t *Trigger) SetEnabledInThreads(enable bool) *Trigger {
	t.EnableInThreads = enable
	return t
}

func (t *Trigger) SetMiddlewares(mw ...MiddleWareFunc) *Trigger {
	t.Middlewares = mw
	return t
}
