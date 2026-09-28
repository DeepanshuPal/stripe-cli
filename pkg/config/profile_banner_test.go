package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-cli/pkg/keyring"
)

func TestActiveContextBannerNeverMixesProfileAndActiveAccount(t *testing.T) {
	tests := []struct {
		name      string
		profileID string
		display   string
		want      string
		forbidden string
	}{
		{"different account", "acct_A", "Account A", "acct_B · sandbox (acct_B)", "Account A"},
		{"matching account", "acct_B", "Account B", "Account B · sandbox (acct_B)", ""},
		{"unknown profile account", "", "Account A", "acct_B · sandbox (acct_B)", "Account A"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			originalStore := KeyRing
			oldStderr := os.Stderr
			t.Cleanup(func() { KeyRing = originalStore; os.Stderr = oldStderr })
			active, err := json.Marshal(ActiveContext{AccountID: "acct_B"})
			require.NoError(t, err)
			KeyRing = keyring.NewMemoryStore(map[string][]byte{
				UATKeychainItemKey:            []byte("oak_test"),
				OAuthActiveContextKeychainKey: active,
			})
			printActiveContextOnce = sync.Once{}
			t.Cleanup(func() { printActiveContextOnce = sync.Once{} })
			reader, writer, err := os.Pipe()
			require.NoError(t, err)
			os.Stderr = writer
			p := &Profile{ProfileName: "account_a", AccountID: tc.profileID}
			configPath := filepath.Join(t.TempDir(), "config.toml")
			require.NoError(t, os.WriteFile(configPath, []byte("[account_a]\ndisplay_name = '"+tc.display+"'\n"), 0600))
			viper.Reset()
			viper.SetConfigFile(configPath)
			t.Cleanup(viper.Reset)
			p.PrintActiveContextBanner()
			writer.Close()
			out, err := io.ReadAll(reader)
			require.NoError(t, err)
			reader.Close()
			require.Contains(t, string(out), tc.want)
			if tc.forbidden != "" {
				require.False(t, strings.Contains(string(out), tc.forbidden))
			}
		})
	}
}
