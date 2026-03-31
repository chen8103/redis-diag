package config

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/url"
)

type ResolvedTarget struct {
	Addr      string
	Username  string
	Password  string
	DB        int
	UseTLS    bool
	TLSConfig *tls.Config
	Display   string
}

func ResolveTarget(cfg Config) (ResolvedTarget, error) {
	resolved := ResolvedTarget{
		Addr:     net.JoinHostPort(cfg.Target.Host, fmt.Sprintf("%d", cfg.Target.Port)),
		Username: cfg.Target.Username,
		Password: cfg.Target.Password,
		DB:       0,
		Display:  net.JoinHostPort(cfg.Target.Host, fmt.Sprintf("%d", cfg.Target.Port)),
	}
	if cfg.Target.URI != "" {
		parsed, err := url.Parse(cfg.Target.URI)
		if err != nil {
			return ResolvedTarget{}, err
		}
		if parsed.Host != "" {
			resolved.Addr = parsed.Host
			resolved.Display = parsed.Host
		}
		if parsed.User != nil {
			resolved.Username = parsed.User.Username()
			if password, ok := parsed.User.Password(); ok {
				resolved.Password = password
			}
		}
		if parsed.Path != "" && parsed.Path != "/" {
			var db int
			_, _ = fmt.Sscanf(parsed.Path, "/%d", &db)
			resolved.DB = db
		}
		if parsed.Scheme == "rediss" {
			resolved.UseTLS = true
		}
	}
	if cfg.Target.TLS {
		resolved.UseTLS = true
	}
	if resolved.UseTLS {
		resolved.TLSConfig = &tls.Config{InsecureSkipVerify: cfg.Target.Insecure} //nolint:gosec
		if cfg.Target.Insecure {
			log.Printf("WARNING: TLS certificate verification is disabled. This is insecure and should only be used for testing.")
		}
	}
	return resolved, nil
}
