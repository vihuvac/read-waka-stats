package logging_test

import (
	"testing"

	"github.com/vihuvac/read-waka-stats/internal/logging"
)

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

func TestLoggerDoesNotPanic(t *testing.T) {
	log := logging.New(true)
	log.Info("info %s", "x")
	log.Warn("warn")
	log.Error("err")
	log.Success("ok")
	_ = logging.New(false)
}
