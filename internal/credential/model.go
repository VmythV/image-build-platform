package credential

import "time"

const (
	TypeRegistryPassword = "registry_password"
	TypeSSHPrivateKey    = "ssh_private_key"

	EncryptionVersion = 1
)

type Credential struct {
	ID                string
	Type              string
	Name              string
	EncryptedValue    string
	EncryptionVersion int
	Fingerprint       string
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
