package ui

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"sqlterm/internal/config"
	"sqlterm/internal/credentials"
)

// Write a new credential before atomically replacing the profile. A failed
// config write removes the new secret and leaves the previous secret intact.
func (u *UI) persistProfile(index int, p config.Profile, password string, replaceSecret bool) error {
	next := u.cfg
	next.Profiles = append([]config.Profile(nil), u.cfg.Profiles...)
	oldID := ""
	if index >= 0 {
		oldID = u.cfg.Profiles[index].CredentialID
	}
	if index >= 0 && endpointChangeRequiresPassword(u.cfg.Profiles[index], p, password) {
		return fmt.Errorf("%s", endpointChangedPasswordRequired)
	}
	newID := ""
	if replaceSecret {
		newID = uuid.NewString()
		if err := u.vault.Set(newID, password); err != nil {
			return fmt.Errorf("cannot save password in system credential store: %w", err)
		}
		p.CredentialID = newID
		p.PasswordEnv = ""
	}
	if index < 0 {
		next.Profiles = append(next.Profiles, p)
	} else {
		next.Profiles[index] = p
	}
	if err := config.Save(u.configPath, next); err != nil {
		if newID != "" {
			_ = u.vault.Delete(newID)
		}
		return err
	}
	if index >= 0 {
		old := u.cfg.Profiles[index]
		for _, tab := range u.tabs {
			if tab.Profile.Name == old.Name && tab.Profile.Folder == old.Folder {
				tab.Profile.Name = p.Name
				tab.Profile.Folder = p.Folder
			}
		}
	}
	u.cfg = next
	if oldID != "" && newID != "" && oldID != newID {
		_ = u.vault.Delete(oldID)
	}
	return nil
}
func (u *UI) rememberPassword(p config.Profile, password string) error {
	for i, profile := range u.cfg.Profiles {
		if profile.Name == p.Name && profile.Folder == p.Folder {
			return u.persistProfile(i, profile, password, true)
		}
	}
	return fmt.Errorf("connection profile no longer exists")
}
func (u *UI) resolvePassword(p config.Profile) (string, bool, error) {
	if p.CredentialID != "" {
		password, err := u.vault.Get(p.CredentialID)
		if err == nil {
			return password, true, nil
		}
		if !errors.Is(err, credentials.ErrNotFound) {
			return "", false, fmt.Errorf("cannot read saved password: %w", err)
		}
	}
	password, ok := p.Password()
	return password, ok, nil
}
