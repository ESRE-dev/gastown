package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrNotFound indicates the config file does not exist.
	ErrNotFound = errors.New("config file not found")

	// ErrInvalidVersion indicates an unsupported schema version.
	ErrInvalidVersion = errors.New("unsupported config version")

	// ErrInvalidType indicates an unexpected config type.
	ErrInvalidType = errors.New("invalid config type")

	// ErrMissingField indicates a required field is missing.
	ErrMissingField = errors.New("missing required field")
)

// LoadTownConfig loads and validates a town configuration file.
func LoadTownConfig(path string) (*TownConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var config TownConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validateTownConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveTownConfig saves a town configuration to a file.
func SaveTownConfig(path string, config *TownConfig) error {
	if err := validateTownConfig(config); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// LoadRigsConfig loads and validates a rigs registry file.
func LoadRigsConfig(path string) (*RigsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var config RigsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validateRigsConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveRigsConfig saves a rigs registry to a file.
func SaveRigsConfig(path string, config *RigsConfig) error {
	if err := validateRigsConfig(config); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// LoadAgentState loads an agent state file.
func LoadAgentState(path string) (*AgentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading state: %w", err)
	}

	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}

	if err := validateAgentState(&state); err != nil {
		return nil, err
	}

	return &state, nil
}

// SaveAgentState saves an agent state to a file.
func SaveAgentState(path string, state *AgentState) error {
	if err := validateAgentState(state); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}

	return nil
}

// validateTownConfig validates a TownConfig.
func validateTownConfig(c *TownConfig) error {
	if c.Type != "town" && c.Type != "" {
		return fmt.Errorf("%w: expected type 'town', got '%s'", ErrInvalidType, c.Type)
	}
	if c.Version > CurrentTownVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentTownVersion)
	}
	if c.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingField)
	}
	return nil
}

// validateRigsConfig validates a RigsConfig.
func validateRigsConfig(c *RigsConfig) error {
	if c.Version > CurrentRigsVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentRigsVersion)
	}
	if c.Rigs == nil {
		c.Rigs = make(map[string]RigEntry)
	}
	return nil
}

// validateAgentState validates an AgentState.
func validateAgentState(s *AgentState) error {
	if s.Role == "" {
		return fmt.Errorf("%w: role", ErrMissingField)
	}
	return nil
}

// LoadRigConfig loads and validates a rig configuration file.
func LoadRigConfig(path string) (*RigConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var config RigConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validateRigConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveRigConfig saves a rig configuration to a file.
func SaveRigConfig(path string, config *RigConfig) error {
	if err := validateRigConfig(config); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// validateRigConfig validates a RigConfig (identity only).
func validateRigConfig(c *RigConfig) error {
	if c.Type != "rig" && c.Type != "" {
		return fmt.Errorf("%w: expected type 'rig', got '%s'", ErrInvalidType, c.Type)
	}
	if c.Version > CurrentRigConfigVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentRigConfigVersion)
	}
	if c.Name == "" {
		return fmt.Errorf("%w: name", ErrMissingField)
	}
	return nil
}

// validateRigSettings validates a RigSettings.
func validateRigSettings(c *RigSettings) error {
	if c.Type != "rig-settings" && c.Type != "" {
		return fmt.Errorf("%w: expected type 'rig-settings', got '%s'", ErrInvalidType, c.Type)
	}
	if c.Version > CurrentRigSettingsVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentRigSettingsVersion)
	}
	if c.MergeQueue != nil {
		if err := validateMergeQueueConfig(c.MergeQueue); err != nil {
			return err
		}
	}
	return nil
}

// ErrInvalidOnConflict indicates an invalid on_conflict strategy.
var ErrInvalidOnConflict = errors.New("invalid on_conflict strategy")

// validateMergeQueueConfig validates a MergeQueueConfig.
func validateMergeQueueConfig(c *MergeQueueConfig) error {
	// Validate on_conflict strategy
	if c.OnConflict != "" && c.OnConflict != OnConflictAssignBack && c.OnConflict != OnConflictAutoRebase {
		return fmt.Errorf("%w: got '%s', want '%s' or '%s'",
			ErrInvalidOnConflict, c.OnConflict, OnConflictAssignBack, OnConflictAutoRebase)
	}

	// Validate poll_interval if specified
	if c.PollInterval != "" {
		if _, err := time.ParseDuration(c.PollInterval); err != nil {
			return fmt.Errorf("invalid poll_interval: %w", err)
		}
	}

	// Validate non-negative values
	if c.RetryFlakyTests < 0 {
		return fmt.Errorf("%w: retry_flaky_tests must be non-negative", ErrMissingField)
	}
	if c.MaxConcurrent < 0 {
		return fmt.Errorf("%w: max_concurrent must be non-negative", ErrMissingField)
	}

	return nil
}

// NewRigConfig creates a new RigConfig (identity only).
func NewRigConfig(name, gitURL string) *RigConfig {
	return &RigConfig{
		Type:    "rig",
		Version: CurrentRigConfigVersion,
		Name:    name,
		GitURL:  gitURL,
	}
}

// NewRigSettings creates a new RigSettings with defaults.
func NewRigSettings() *RigSettings {
	return &RigSettings{
		Type:       "rig-settings",
		Version:    CurrentRigSettingsVersion,
		MergeQueue: DefaultMergeQueueConfig(),
		Namepool:   DefaultNamepoolConfig(),
	}
}

// LoadRigSettings loads and validates a rig settings file.
func LoadRigSettings(path string) (*RigSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	var settings RigSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings: %w", err)
	}

	if err := validateRigSettings(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

// SaveRigSettings saves rig settings to a file.
func SaveRigSettings(path string, settings *RigSettings) error {
	if err := validateRigSettings(settings); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	return nil
}

// LoadMayorConfig loads and validates a mayor config file.
func LoadMayorConfig(path string) (*MayorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var config MayorConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validateMayorConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveMayorConfig saves a mayor config to a file.
func SaveMayorConfig(path string, config *MayorConfig) error {
	if err := validateMayorConfig(config); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// validateMayorConfig validates a MayorConfig.
func validateMayorConfig(c *MayorConfig) error {
	if c.Type != "mayor-config" && c.Type != "" {
		return fmt.Errorf("%w: expected type 'mayor-config', got '%s'", ErrInvalidType, c.Type)
	}
	if c.Version > CurrentMayorConfigVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentMayorConfigVersion)
	}
	return nil
}

// NewMayorConfig creates a new MayorConfig with defaults.
func NewMayorConfig() *MayorConfig {
	return &MayorConfig{
		Type:    "mayor-config",
		Version: CurrentMayorConfigVersion,
	}
}

// LoadAccountsConfig loads and validates an accounts configuration file.
func LoadAccountsConfig(path string) (*AccountsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("reading accounts config: %w", err)
	}

	var config AccountsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing accounts config: %w", err)
	}

	if err := validateAccountsConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveAccountsConfig saves an accounts configuration to a file.
func SaveAccountsConfig(path string, config *AccountsConfig) error {
	if err := validateAccountsConfig(config); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding accounts config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing accounts config: %w", err)
	}

	return nil
}

// validateAccountsConfig validates an AccountsConfig.
func validateAccountsConfig(c *AccountsConfig) error {
	if c.Version > CurrentAccountsVersion {
		return fmt.Errorf("%w: got %d, max supported %d", ErrInvalidVersion, c.Version, CurrentAccountsVersion)
	}
	if c.Accounts == nil {
		c.Accounts = make(map[string]Account)
	}
	// Validate default refers to an existing account (if set and accounts exist)
	if c.Default != "" && len(c.Accounts) > 0 {
		if _, ok := c.Accounts[c.Default]; !ok {
			return fmt.Errorf("%w: default account '%s' not found in accounts", ErrMissingField, c.Default)
		}
	}
	// Validate each account has required fields
	for handle, acct := range c.Accounts {
		if acct.ConfigDir == "" {
			return fmt.Errorf("%w: config_dir for account '%s'", ErrMissingField, handle)
		}
	}
	return nil
}

// NewAccountsConfig creates a new AccountsConfig with defaults.
func NewAccountsConfig() *AccountsConfig {
	return &AccountsConfig{
		Version:  CurrentAccountsVersion,
		Accounts: make(map[string]Account),
	}
}

// GetAccount returns an account by handle, or nil if not found.
func (c *AccountsConfig) GetAccount(handle string) *Account {
	if acct, ok := c.Accounts[handle]; ok {
		return &acct
	}
	return nil
}

// GetDefaultAccount returns the default account, or nil if not set.
func (c *AccountsConfig) GetDefaultAccount() *Account {
	if c.Default == "" {
		return nil
	}
	return c.GetAccount(c.Default)
}

// ResolveAccountConfigDir resolves the CLAUDE_CONFIG_DIR for account selection.
// Priority order:
//  1. GT_ACCOUNT environment variable
//  2. accountFlag (from --account command flag)
//  3. Default account from config
//
// Returns empty string if no account configured or resolved.
// Returns the handle that was resolved as second value.
func ResolveAccountConfigDir(accountsPath, accountFlag string) (configDir, handle string, err error) {
	// Load accounts config
	cfg, loadErr := LoadAccountsConfig(accountsPath)
	if loadErr != nil {
		// No accounts configured - that's OK, return empty
		return "", "", nil
	}

	// Priority 1: GT_ACCOUNT env var
	if envAccount := os.Getenv("GT_ACCOUNT"); envAccount != "" {
		acct := cfg.GetAccount(envAccount)
		if acct == nil {
			return "", "", fmt.Errorf("GT_ACCOUNT '%s' not found in accounts config", envAccount)
		}
		return expandPath(acct.ConfigDir), envAccount, nil
	}

	// Priority 2: --account flag
	if accountFlag != "" {
		acct := cfg.GetAccount(accountFlag)
		if acct == nil {
			return "", "", fmt.Errorf("account '%s' not found in accounts config", accountFlag)
		}
		return expandPath(acct.ConfigDir), accountFlag, nil
	}

	// Priority 3: Default account
	if cfg.Default != "" {
		acct := cfg.GetDefaultAccount()
		if acct != nil {
			return expandPath(acct.ConfigDir), cfg.Default, nil
		}
	}

	return "", "", nil
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
