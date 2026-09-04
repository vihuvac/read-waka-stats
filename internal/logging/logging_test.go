package logging_test

import (
	"io"
	"os"
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/logging"
)

func TestMain(m *testing.M) {
	logging.Output = io.Discard
	os.Exit(m.Run())
}

func TestParseBoolTruth(t *testing.T) {
	for _, v := range []string{"true", "TRUE", "1", "yes", "Y", "t"} {
		if !logging.ParseBoolTruth(v) {
			t.Fatalf("%q should be true", v)
		}
	}
	for _, v := range []string{"false", "0", "", "no"} {
		if logging.ParseBoolTruth(v) {
			t.Fatalf("%q should be false", v)
		}
	}
}

func TestLoggerLevels(t *testing.T) {
	quiet := logging.New(false)
	quiet.Info("hidden")
	quiet.Warn("warn")
	quiet.Error("err")
	quiet.Success("ok")

	debug := logging.New(true)
	debug.Info("shown")
	debug.Warn("warn")
	debug.Error("err")
	debug.Success("ok")
}

func TestFatalUsesExitFunc(t *testing.T) {
	orig := logging.ExitFunc
	t.Cleanup(func() { logging.ExitFunc = orig })
	code := -1
	logging.ExitFunc = func(c int) { code = c }
	logging.New(false).Fatal("boom %s", "x")
	if code != 1 {
		t.Fatalf("exit code=%d", code)
	}
}
