package packageimport

type pullConfig struct {
	Insecure   bool
	PullSecret []byte
}

func (c *pullConfig) Option(opts ...PullOption) {
	for _, opt := range opts {
		opt.ConfigurePull(c)
	}
}

type PullOption interface {
	ConfigurePull(*pullConfig)
}

type WithPullSecret struct {
	Data []byte
}

func (wp WithPullSecret) ConfigurePull(cfg *pullConfig) {
	cfg.PullSecret = wp.Data
}

type WithInsecure bool

func (w WithInsecure) ConfigurePull(c *pullConfig) {
	c.Insecure = bool(w)
}
