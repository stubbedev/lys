package config

// DrivesConfig is the drives section: what the session does with the
// drives plugged into it.
//
// Removable drives.
type DrivesConfig struct {
	// Mount the filesystems on removable drives, such as USB sticks and
	// SD cards, as they are plugged in, and those already plugged in at
	// login, with a notification that opens or ejects each.
	//
	// Internal drives are never mounted this way, and encrypted volumes
	// stay locked until unlocked in the file manager.
	Automount bool `cfg:"automount"`
}

// DefaultsDrives returns the schema defaults: removable drives mount as
// they are plugged in.
func DefaultsDrives() DrivesConfig {
	return DrivesConfig{Automount: true}
}
