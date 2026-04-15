package deployments

import "testing"

func TestBuildFTPOptionsUsesConfiguredPassiveNetworking(t *testing.T) {
	manager := &Manager{
		ftpPublicHost:   "127.0.0.1",
		ftpPassivePorts: "9100-9199",
	}

	opts := manager.buildFTPOptions(Deployment{
		ID:   "deployment-ftp-1",
		Port: 9002,
	}, &ftpFileDriver{root: t.TempDir()})

	if opts.PublicIP != "127.0.0.1" {
		t.Fatalf("PublicIP = %q, want %q", opts.PublicIP, "127.0.0.1")
	}
	if opts.PassivePorts != "9100-9199" {
		t.Fatalf("PassivePorts = %q, want %q", opts.PassivePorts, "9100-9199")
	}
	if opts.Port != 9002 {
		t.Fatalf("Port = %d, want %d", opts.Port, 9002)
	}
}
