//go:build !nvidia

package gpu

func New() Collector { return Noop() }
