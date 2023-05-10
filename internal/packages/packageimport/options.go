package packageimport

type WithInsecure bool

func (w WithInsecure) ConfigurePull(c *PullConfig) {
	c.Insecure = bool(w)
}
