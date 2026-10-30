package messagestore

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/kayushkin/llm-bridge/msg"
	"github.com/kayushkin/llm-bridge/servicesettings"
)

// ServiceName is this service's name in its own settings description, as
// healthcheck and the repo know it.
const ServiceName = "message-store"

// OwnedEnvironmentVariablePrefix is the prefix of the variables that are this
// service's alone. A set variable carrying it that SettingDefinitions does not
// declare stops the service from starting.
const OwnedEnvironmentVariablePrefix = "MESSAGE_STORE_"

// Keys of the settings, as GET /settings names them.
const (
	SettingListenAddress = "listen_address"
	SettingDataDirectory = "data_directory"
	SettingStoreURL      = "message_store_url"
)

// DefaultListenAddress is loopback, deliberately: these are private messages,
// and the service has no auth of its own. dash is the front door.
const DefaultListenAddress = "127.0.0.1:8337"

// DefaultDataDir is where the database lives when the environment says
// nothing.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "message-store")
}

// SettingDefinitions declares every environment variable the service reads,
// once. Nothing is Editable: GET /settings is as open as every other route.
func SettingDefinitions() []servicesettings.Definition {
	return []servicesettings.Definition{
		{Key: SettingListenAddress, EnvironmentVariable: "MESSAGE_STORE_ADDR", Kind: msg.ServiceSettingKindWiring, ValueType: msg.ServiceSettingValueTypeString, Default: DefaultListenAddress,
			Description: "The address the HTTP server listens on. Keep it on loopback: the service has no auth."},
		{Key: SettingDataDirectory, EnvironmentVariable: "MESSAGE_STORE_DATA_DIR", Kind: msg.ServiceSettingKindPath, ValueType: msg.ServiceSettingValueTypeString, Default: DefaultDataDir(),
			Description: "The directory that holds message-store.db. Changing it starts the service on whatever is there, or nothing."},
	}
}

// NewSettingsRegistry reads this service's settings from environment.
func NewSettingsRegistry(environment servicesettings.Environment) (*servicesettings.Registry, error) {
	return servicesettings.New(ServiceName, []string{OwnedEnvironmentVariablePrefix}, SettingDefinitions(), environment)
}

// RegisterSettingsHandler serves the registry at GET /settings.
func RegisterSettingsHandler(mux *http.ServeMux, registry *servicesettings.Registry) {
	mux.Handle("GET /settings", servicesettings.Handler(registry, "/settings"))
}

// StoreURLDefinition is the one setting every importer shares: where to post.
func StoreURLDefinition() servicesettings.Definition {
	return servicesettings.Definition{Key: SettingStoreURL, EnvironmentVariable: "MESSAGE_STORE_URL", Kind: msg.ServiceSettingKindWiring, ValueType: msg.ServiceSettingValueTypeString, Default: "http://127.0.0.1:8337",
		Description: "Where message-store answers: the command posts there."}
}

// ImporterSettingDefinitions declares every variable an importer command
// reads. An importer that reads more adds its own definitions here, so the
// settings test sees them.
func ImporterSettingDefinitions() []servicesettings.Definition {
	return []servicesettings.Definition{StoreURLDefinition()}
}

// NewImporterSettingsRegistry reads an importer's settings. It owns no
// prefix: MESSAGE_STORE_ belongs to the service, and the service's registry
// would refuse MESSAGE_STORE_URL, so the importers never build that one.
func NewImporterSettingsRegistry(importer string, environment servicesettings.Environment) (*servicesettings.Registry, error) {
	return servicesettings.New(importer, nil, ImporterSettingDefinitions(), environment)
}
