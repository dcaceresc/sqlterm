package ui

import "sqlterm/internal/config"

const endpointChangedPasswordRequired = "Endpoint changed; enter the password again"

// Database, display name and folder do not change the credential recipient.
func sameCredentialEndpoint(a, b config.Profile) bool {
	return a.Engine == b.Engine && a.Host == b.Host &&
		a.EffectivePort() == b.EffectivePort() && a.User == b.User
}

func endpointChangeRequiresPassword(original, edited config.Profile, password string) bool {
	return !sameCredentialEndpoint(original, edited) &&
		(original.CredentialID != "" || original.PasswordEnv != "") && password == ""
}
