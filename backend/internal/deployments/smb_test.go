package deployments

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSMBConfigIncludesGuestReadonlyShareAndCustomPort(t *testing.T) {
	deployment := Deployment{
		ID:       "deployment-smb-1",
		Protocol: "smb",
		Port:     1445,
	}
	runDir := filepath.Join("tmp", "honeygen-smb")
	sharePath := filepath.Join("storage", "generated", "northbridge", "job-1", "shared")

	config := buildSMBConfig(deployment, sharePath, runDir)

	for _, want := range []string{
		"smb ports = 1445",
		"server role = standalone server",
		"server min protocol = SMB2",
		"map to guest = Bad User",
		"vfs objects = full_audit",
		"full_audit:syslog = false",
		"full_audit:prefix = deployment-smb-1|%I",
		"full_audit:success = connect disconnect open",
		"[honeygen]",
		"path = " + filepath.ToSlash(sharePath),
		"read only = yes",
		"guest ok = yes",
		"browseable = yes",
		"disable spoolss = yes",
		"log file = " + filepath.ToSlash(filepath.Join(runDir, "log.%m")),
		"ncalrpc dir = " + filepath.ToSlash(filepath.Join(runDir, "ncalrpc")),
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q\nconfig:\n%s", want, config)
		}
	}
}
