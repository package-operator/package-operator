package deps

import (
	"flag"

	"github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"package-operator.run/cmd/kubectl-package/rootcmd"
)

func ProvideLogFactory(streams rootcmd.IOStreams) LogFactory {
	opts := &zap.Options{
		Development:     true,
		DestWriter:      streams.ErrOut,
		Level:           zapcore.ErrorLevel,
		StacktraceLevel: zapcore.ErrorLevel,
	}
	opts.BindFlags(flag.CommandLine)

	return &ZapLogFactory{
		opts: opts,
	}
}

type LogFactory interface {
	Logger() logr.Logger
}

type ZapLogFactory struct {
	opts *zap.Options
}

func (f *ZapLogFactory) Logger() logr.Logger {
	return zap.New(zap.UseFlagOptions(f.opts))
}
