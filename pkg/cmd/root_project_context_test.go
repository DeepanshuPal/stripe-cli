package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/stripe/stripe-cli/pkg/config"
	"github.com/stripe/stripe-cli/pkg/errorcategory"
	"github.com/stripe/stripe-cli/pkg/keyring"
)

func TestExplicitProjectContextGuard(t *testing.T) {
	cases := []struct {
		name, profileName, profileID, activeID string
		flag, env, override                    bool
		command                                string
		blocked                                bool
	}{
		{name: "mismatched flag", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", flag: true, command: "customers", blocked: true},
		{name: "mismatched environment", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", env: true, command: "customers", blocked: true},
		{name: "matching flag", profileName: "account_b", profileID: "acct_A", activeID: "acct_A", flag: true, command: "customers"},
		{name: "unknown profile account", profileName: "account_b", activeID: "acct_A", flag: true, command: "customers"},
		{name: "no explicit selection", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", command: "customers"},
		{name: "explicit API key", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", flag: true, override: true, command: "customers"},
		{name: "switch remains available", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", flag: true, command: "switch"},
		{name: "whoami remains available", profileName: "account_b", profileID: "acct_B", activeID: "acct_A", flag: true, command: "whoami"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldStore := config.KeyRing
			oldFlag := rootCmd.PersistentFlags().Lookup("project-name").Changed
			t.Cleanup(func() {
				config.KeyRing = oldStore
				rootCmd.PersistentFlags().Lookup("project-name").Changed = oldFlag
				viper.Reset()
			})
			rootCmd.PersistentFlags().Lookup("project-name").Changed = tc.flag
			if tc.env {
				t.Setenv("STRIPE_PROJECT_NAME", tc.profileName)
			} else {
				t.Setenv("STRIPE_PROJECT_NAME", "")
			}
			active, err := json.Marshal(config.ActiveContext{AccountID: tc.activeID})
			require.NoError(t, err)
			config.KeyRing = keyring.NewMemoryStore(map[string][]byte{
				config.UATKeychainItemKey:            []byte("oak_test"),
				config.OAuthActiveContextKeychainKey: active,
			})
			profile := &config.Profile{ProfileName: tc.profileName, APIKey: ""}
			if tc.override {
				profile.APIKey = "sk_test_override"
			}
			// Config-backed account ID follows the -p profile, not an injected in-memory field.
			path := filepath.Join(t.TempDir(), "config.toml")
			require.NoError(t, os.WriteFile(path, []byte("["+tc.profileName+"]\naccount_id = '"+tc.profileID+"'\n"), 0600))
			viper.Reset()
			viper.SetConfigFile(path)
			child := &cobra.Command{Use: tc.command}
			rootCmd.AddCommand(child)
			t.Cleanup(func() { rootCmd.RemoveCommand(child) })
			err = validateExplicitProjectContext(child, profile, tc.flag)
			if tc.blocked {
				require.Error(t, err)
				require.Contains(t, err.Error(), "acct_B")
				require.Contains(t, err.Error(), "acct_A")
				category, ok := errorcategory.Get(err)
				require.True(t, ok)
				require.Equal(t, errorcategory.UserInput, category)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
