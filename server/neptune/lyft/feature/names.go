package feature

type Name string

// list of feature names used in the code base. These must be kept in sync with any external config.
const (
	PlatformMode      Name = "platform-mode"
	LegacyDeprecation Name = "legacy-deprecation"
)
