package onboard

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/term"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	dockerConfigDirEnv   = "DOCKER_CONFIG"
	dockerConfigFileDir  = ".docker"
	dockerConfigFileName = "config.json"
)

type LoginOptions struct {
	Registry                   string
	Username                   string
	UserPass                   string
	RegistryConfigPath         string
	FallbackRegistryConfigPath []string

	UsernameSTDIN bool
	PasswordSTDIN bool
	IDTokenSTDIN  bool

	PlainHTTP bool
	Insecure  bool

	store          credentials.Store
	credentialFunc auth.CredentialFunc
}

func getRealUserHome() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	sudoUserID := os.Getenv("SUDO_UID")
	if sudoUserID != "" && usr.Uid != sudoUserID {
		usr, err = user.LookupId(sudoUserID)
		if err != nil {
			return "", err
		}

		return usr.HomeDir, nil
	}

	return "", nil
}

func getDockerConfigPath(customHomeDir string) (string, error) {
	// try provided home directory
	var configDir string
	if customHomeDir != "" {
		configDir = filepath.Join(customHomeDir, dockerConfigFileDir)
	}

	if configDir == "" {
		// try getting from environment variable
		configDir := os.Getenv(dockerConfigDirEnv)

		if configDir == "" {
			// try to get home directory
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get user home directory: %w", err)
			}
			configDir = filepath.Join(homeDir, dockerConfigFileDir)
		}
	}

	return filepath.Join(configDir, dockerConfigFileName), nil
}

func readLine(outWriter io.Writer, prompt string, silent bool) (string, error) {
	_, _ = fmt.Fprint(outWriter, prompt)
	// #nosec G115 -- skip for now
	fd := int(os.Stdin.Fd())
	var bytes []byte
	var err error
	if silent && term.IsTerminal(fd) {
		if bytes, err = term.ReadPassword(fd); err == nil {
			_, err = fmt.Fprintln(outWriter)
		}
	} else {
		bytes, err = ReadLine(os.Stdin)
	}
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func readSecret() (string, error) {
	// Prompt for credential
	secretInput, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	// remove trailing cred
	secret := strings.TrimSuffix(string(secretInput), "\n")
	secret = strings.TrimSuffix(secret, "\r")

	return secret, nil
}

func (lo *LoginOptions) newStore() (credentials.Store, error) {
	var (
		store credentials.Store
		err   error
	)

	// initialize credentials store
	storeOpts := credentials.StoreOptions{AllowPlaintextPut: true}
	if lo.RegistryConfigPath == "" {
		// use default docker config file path
		store, err = credentials.NewStoreFromDocker(storeOpts)
		if err != nil {
			return nil, err
		}
	} else {
		store, err = credentials.NewStore(lo.RegistryConfigPath, storeOpts)
		if err != nil {
			return nil, err
		}
	}

	if len(lo.FallbackRegistryConfigPath) != 0 {
		fallbackStores := make([]credentials.Store, 0)
		for _, configPath := range lo.FallbackRegistryConfigPath {
			newStore, err := credentials.NewStore(configPath, storeOpts)
			if err != nil {
				return nil, err
			}

			fallbackStores = append(fallbackStores, newStore)
		}

		return credentials.NewStoreWithFallbacks(store, fallbackStores...), nil
	}

	return store, nil
}

func (lo *LoginOptions) isPlainHttp(registry string) bool {
	if lo.PlainHTTP {
		return lo.PlainHTTP
	}

	host, _, _ := net.SplitHostPort(registry)
	if host == "localhost" || registry == "localhost" {
		// not specified, defaults to plain http for localhost
		return true
	}

	return false
}

func (lo *LoginOptions) authClient() (client *auth.Client, err error) {
	// ignoring G402 - upto user to use secure connection
	config := &tls.Config{
		InsecureSkipVerify: lo.Insecure, // #nosec G402
	}

	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = config

	client = &auth.Client{
		Client: &http.Client{
			// http.RoundTripper with a retry using the DefaultPolicy
			// see: https://pkg.go.dev/oras.land/oras-go/v2/registry/remote/retry#Policy
			Transport: retry.NewTransport(baseTransport),
		},
		Cache:      auth.NewCache(),
		Credential: lo.credentialFunc,
	}

	cred := Credential(lo.Username, lo.UserPass)
	if cred != auth.EmptyCredential {
		client.Credential = func(ctx context.Context, s string) (auth.Credential, error) {
			return cred, nil
		}
	} else if lo.store != nil {
		client.Credential = credentials.Credential(lo.store)
	} else {
		store, err := lo.newStore()
		if err != nil {
			return nil, err
		}
		client.Credential = credentials.Credential(store)
	}

	return
}

func (lo *LoginOptions) newRegistry() (reg *remote.Registry, err error) {
	reg, err = remote.NewRegistry(lo.Registry)
	if err != nil {
		return nil, err
	}

	registry := reg.Reference.Registry
	reg.PlainHTTP = lo.isPlainHttp(registry)

	if reg.Client, err = lo.authClient(); err != nil {
		return nil, err
	}

	return
}

// Credential returns a credential based on the remote options.
func Credential(username, password string) auth.Credential {
	if username == "" {
		return auth.Credential{
			RefreshToken: password,
		}
	}
	return auth.Credential{
		Username: username,
		Password: password,
	}
}

func (lo *LoginOptions) ORASGetAuthClient() (*auth.Client, error) {
	var err error

	// to account for a user signing in without sudo
	if realUserHome, err := getRealUserHome(); err != nil {
		return nil, err
	} else if realUserHome != "" {
		dockerConfigPath, err := getDockerConfigPath(realUserHome)
		if err != nil {
			return nil, err
		}

		lo.FallbackRegistryConfigPath = append(lo.FallbackRegistryConfigPath, dockerConfigPath)
	}

	lo.store, err = lo.newStore()
	if err != nil {
		return nil, err
	}

	lo.credentialFunc = credentials.Credential(lo.store)

	// initialize registry
	authClient, err := lo.authClient()
	if err != nil {
		return nil, err
	}
	return authClient, nil
}

func (lo *LoginOptions) ORASRegistryLogin() error {
	var err error

	// if registry config path was specified it would be handled by newStore
	if lo.RegistryConfigPath == "" {
		if lo.IDTokenSTDIN || (lo.Username == "" && !lo.UsernameSTDIN && lo.UserPass != "") {
			lo.UserPass, err = readLine(os.Stdin, "ID TOKEN: ", true)
			if err != nil {
				return err
			}
		} else {
			if lo.Username == "" || lo.UsernameSTDIN {
				lo.Username, err = readLine(os.Stdin, "Username: ", false)
				if err != nil {
					return err
				}
			}

			if lo.UserPass == "" || lo.PasswordSTDIN {
				lo.UserPass, err = readLine(os.Stdin, "Password: ", true)
				if err != nil {
					return err
				}
			}
		}
	}

	lo.store, err = lo.newStore()
	if err != nil {
		return err
	}

	lo.credentialFunc = credentials.Credential(lo.store)

	// initialize registry
	remote, err := lo.newRegistry()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err = credentials.Login(ctx, lo.store, remote, Credential(lo.Username, lo.UserPass)); err != nil {
		return err
	}

	return nil
}
