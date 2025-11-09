// Assumes CGO is enabled.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

type AppInfo struct {
	Name    string
	Version string
	BaseURL string // e.g., "https://example.com/"
	Storage string // path to storage directory
	Runtime string // path to runtime directory (XDG_RUNTIME_DIR/name, fallback to /tmp/name-USER)
}

type ctxKey struct{}

func IntoContext(ctx context.Context, appInfo AppInfo) context.Context {
	return context.WithValue(ctx, ctxKey{}, appInfo)
}

func FromContext(ctx context.Context) (AppInfo, bool) {
	appInfo, ok := ctx.Value(ctxKey{}).(AppInfo)
	return appInfo, ok && (appInfo != AppInfo{})
}

// NewAppInfo creates a new AppInfo instance with default values.
// It calculates Storage and Runtime paths based on standard conventions.
func NewAppInfo(name, version string) (AppInfo, error) {
	appInfo := AppInfo{
		Name:    name,
		Version: version,
	}
	var err error

	if appInfo.Storage, err = getStoragePath(appInfo.Name); err != nil {
		return AppInfo{}, err
	}
	if appInfo.Runtime, err = getRuntimePath(appInfo.Name); err != nil {
		return AppInfo{}, err
	}

	return appInfo, nil
}

// getStoragePath calculates the storage path for the application (~/.appName).
func getStoragePath(appName string) (string, error) {
	// non-root: use current user's home.
	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home dir: %w", err)
		}
		return filepath.Join(home, "."+appName), nil
	}

	// root: require an invoking non-root user (sudo/doas).
	home, err := invokingUserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+appName), nil
}

// getRuntimePath calculates the runtime path for the application.
// Prefers XDG_RUNTIME_DIR, falls back to /tmp/appName-USER.
func getRuntimePath(appName string) (string, error) {
	// prefer XDG_RUNTIME_DIR (typically /run/user/UID)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, appName), nil
	}

	// fallback for non-systemd systems
	// include username to avoid conflicts in shared /tmp
	username := os.Getenv("USER")
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("cannot determine current user: %w", err)
		}
		username = u.Username
	}

	return filepath.Join("/tmp", appName+"-"+username), nil
}

func invokingUserHome() (string, error) {
	// prefer UID (avoids name ambiguities).
	if uid := firstNonEmpty(os.Getenv("SUDO_UID"), os.Getenv("DOAS_UID")); uid != "" && uid != "0" {
		u, err := user.LookupId(uid)
		if err != nil {
			return "", fmt.Errorf("cannot lookup uid %s: %w", uid, err)
		}
		if u.HomeDir == "" {
			return "", fmt.Errorf("empty home for uid %s", uid)
		}
		return u.HomeDir, nil
	}

	// fallback to username if UID not present.
	if uname := firstNonEmpty(os.Getenv("SUDO_USER"), os.Getenv("DOAS_USER")); uname != "" {
		u, err := user.Lookup(uname)
		if err != nil {
			return "", fmt.Errorf("cannot lookup user %q: %w", uname, err)
		}
		if u.Uid == "0" {
			return "", errors.New("invoking user resolves to root; aborting")
		}
		if u.HomeDir == "" {
			return "", fmt.Errorf("empty home for user %q", uname)
		}
		return u.HomeDir, nil
	}

	return "", errors.New("refusing to run as real root: no SUDO_*/DOAS_* env present")
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
