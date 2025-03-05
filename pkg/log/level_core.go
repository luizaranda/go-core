package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// coreWithLevel struct wraps a zapcore.Core and enables dynamic change of
// the logging level.
//
// Given how zap core works, the level can only be changed to a more restrictive
// one, this means that if the core level is set to Warn, we can use coreWithLevel
// to only log Error and above, but we can't use it to log Debug or Info entries.
//
// Considering this restriction the recommended way of using coreWithLevel is to
// wrap a core that can log everything, in essence, use a core with Debug level
// enabled and restrict the logging level using coreWithLevel.
type coreWithLevel struct {
	zapcore.Core

	lvl *zap.AtomicLevel
}

// Enabled returns true if the given level is at or above the configured
// level of both the wrapper & core.
func (c *coreWithLevel) Enabled(level zapcore.Level) bool {
	return c.lvl.Enabled(level) && c.Core.Enabled(level)
}

// Check determines whether the supplied Entry should be logged (using
// the embedded LevelEnabler and possibly some extra logic).
func (c *coreWithLevel) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// Check works by checking if the given entry is loggable with the logger
	// level, if it's not then it returns nil. The `ce` param is by default
	// nil and if we want to log we must create a new zapcore.CheckedEntry.
	//
	// Unfortunately instantiation of this struct happens within zaps ioCore which
	// is private, so we must delegate to the wrapped c.Core. The wrapped core
	// will only return non-nil if it is set at a level that accepts the entry
	// being given, this is the reason of the limitation that coreWithLevel
	// only works further limiting logging level, and does not allow to
	// be more flexible than the wrapped core.
	if !c.lvl.Enabled(e.Level) {
		return ce
	}
	return c.Core.Check(e, ce)
}

// With adds structured context to the Core. Given how zap works internally (it
// returns a new private ioCore) new must wrap again the given core
// within a coreWithLevel.
func (c *coreWithLevel) With(fields []zapcore.Field) zapcore.Core {
	core := c.Core.With(fields)
	return &coreWithLevel{
		Core: core,
		lvl:  c.lvl,
	}
}

// wrapCoreWithLevel returns a zap.Option to use with zap.logger.WithOption
// method which wraps the current zap.logger core within a coreWithLevel
// with the new given level.
func wrapCoreWithLevel(l *zap.AtomicLevel) zap.Option {
	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		newCore := &coreWithLevel{
			Core: core,
			lvl:  l,
		}

		// If core is a coreWithLevel we want to wrap the underlying core.
		// The underlying core should be configured at Debug level.
		lvlCore, ok := core.(*coreWithLevel)
		if ok {
			newCore.Core = lvlCore.Core
		}

		return newCore
	})
}
