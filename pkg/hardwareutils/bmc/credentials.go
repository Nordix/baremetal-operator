package bmc

const (
	SSHWakeupEnabled = "enabled"
)

// Credentials holds the information for authenticating with the BMC.
type Credentials struct {
	Username   string
	Password   string
	SSHWakeup  string
	SSHUser    string
	SSHAddress string
	SSHKey     string
}

// Validate returns an error if the credentials are invalid.
func (creds Credentials) Validate() error {
	if creds.Username == "" {
		return &CredentialsValidationError{message: "Missing BMC connection detail 'username' in credentials"}
	}
	if creds.Password == "" {
		return &CredentialsValidationError{message: "Missing BMC connection details 'password' in credentials"}
	}
	if creds.SSHWakeup == SSHWakeupEnabled {
		if (creds.SSHUser == "") || (creds.SSHAddress == "") || (creds.SSHKey == "") {
			return &CredentialsValidationError{message: "Missing SSH wakeup credentials please provide SSHUser, SSHAddress, SSHKey."}
		}
	} else {
		if (creds.SSHUser != "") || (creds.SSHAddress != "") || (creds.SSHKey != "") {
			return &CredentialsValidationError{message: "SSHWakeup is NOT set to true yet there are SSH credentials provided."}
		}
	}

	return nil
}
