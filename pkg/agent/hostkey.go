package agent

const hostPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDGvmllKUGUEeQkEHv2fcnQqrNfnCzzWDJqMBoj670RkgAAAJBofHUjaHx1
IwAAAAtzc2gtZWQyNTUxOQAAACDGvmllKUGUEeQkEHv2fcnQqrNfnCzzWDJqMBoj670Rkg
AAAEDqp+VwQ9W86otCoRuUQ1eKytgG+3HxBU/eHFPJeC/2mca+aWUpQZQR5CQQe/Z9ydCq
s1+cLPNYMmowGiPrvRGSAAAABnNzaG9sZQECAwQFBgc=
-----END OPENSSH PRIVATE KEY-----`

// hostPublicKey is the ssh-ed25519 public key derived from hostPrivateKey.
const hostPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMa+aWUpQZQR5CQQe/Z9ydCqs1+cLPNYMmowGiPrvRGS sshole"

// HostPublicKey exposes the public host key for writing known_hosts entries.
func HostPublicKey() string {
	return hostPublicKey
}
