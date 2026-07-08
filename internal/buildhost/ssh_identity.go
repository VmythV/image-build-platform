package buildhost

import (
	"fmt"
	"os"
	"strings"
)

func PrepareSSHIdentity(host BuildHost) (BuildHost, func(), error) {
	if host.SSHCredential == nil || strings.TrimSpace(host.SSHCredential.PrivateKey) == "" {
		return host, func() {}, nil
	}

	file, err := os.CreateTemp("", "ibp-ssh-key-*")
	if err != nil {
		return BuildHost{}, nil, fmt.Errorf("create SSH identity file: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return BuildHost{}, nil, fmt.Errorf("secure SSH identity file: %w", err)
	}

	privateKey := strings.TrimSpace(host.SSHCredential.PrivateKey) + "\n"
	if _, err := file.WriteString(privateKey); err != nil {
		_ = file.Close()
		cleanup()
		return BuildHost{}, nil, fmt.Errorf("write SSH identity file: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return BuildHost{}, nil, fmt.Errorf("close SSH identity file: %w", err)
	}

	host.SSHIdentityFile = path
	return host, cleanup, nil
}
