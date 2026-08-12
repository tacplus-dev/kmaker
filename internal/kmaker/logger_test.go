package kmaker

import (
	"os"
	"strings"
	"testing"
)

// TestRunInstallsTheChassisLogger guards the one service in the fleet that
// bypassed credential redaction entirely.
//
// kmaker is hand-written rather than generated, so the sweep that moved every
// generated service onto the chassis logger did not touch it: Run() called
// zap.NewProduction() directly, which has no redaction of any kind. That is the
// worst service to leave uncovered — it holds the signing CA and mints keys, so
// its error paths are the ones most likely to carry key material, and the
// chassis redactor is what stops a credential reaching the log through a
// structured FIELD rather than the message text.
//
// Asserted against the source because installation happens inside Run(), which
// binds sockets and needs a cluster; a behavioural test would be an integration
// test, and this catches the exact regression (someone reaching for
// zap.NewProduction) at the point it would be introduced.
func TestRunInstallsTheChassisLogger(t *testing.T) {
	src, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)

	if strings.Contains(body, "zap.NewProduction()") {
		t.Error("Run() uses zap.NewProduction(), which performs no credential redaction; " +
			"use loggerchassis.InstallFromConfig")
	}
	if !strings.Contains(body, "loggerchassis.InstallFromConfig") {
		t.Error("Run() must install the chassis logger, which redacts structured fields")
	}
	// The configured logger must also come from the chassis, not be built ad hoc
	// once the config is known.
	if !strings.Contains(body, "loggerchassis.InstallFromConfig(cfg.GetLogger())") {
		t.Error("the service's own logger config must be installed through the chassis too")
	}
}
