package internal

import (
	"encoding/json"
	"log/syslog"
)

type AttributeMap struct {
	Source        string
	Destination   string
	Required      bool
	CaseSensitive bool
	Expression    string // regular expression
	Replace       string // replace string
}

type SourceConfig struct {
	Type      string
	ExtraJSON json.RawMessage
}

type DestinationConfig struct {
	Type          string
	ExtraJSON     json.RawMessage
	DisableAdd    bool
	DisableUpdate bool
	DisableDelete bool
}

const (
	VerbosityLow    = 0
	VerbosityMedium = 5
	VerbosityHigh   = 10
)

type RuntimeConfig struct {
	DryRunMode  bool
	Environment string
	SentryDSN   string
	SentryTags  map[string]string
	Verbosity   int
}

type SyncSet struct {
	Name        string
	Source      json.RawMessage
	Destination json.RawMessage
	Disable     bool
}

type ChangeSet struct {
	Create []Person
	Update []Person
	Delete []Person
}

type ChangeResults struct {
	Created uint64
	Updated uint64
	Deleted uint64
}

type EventLogItem struct {
	Message string
	Level   syslog.Priority
}

func (l EventLogItem) String() string {
	return LogLevels[l.Level] + ": " + l.Message
}

func (l EventLogItem) Error() string {
	return LogLevels[l.Level] + ": " + l.Message
}

var LogLevels = map[syslog.Priority]string{
	syslog.LOG_EMERG:   "Emerg",
	syslog.LOG_ALERT:   "Alert",
	syslog.LOG_CRIT:    "Critical",
	syslog.LOG_ERR:     "Error",
	syslog.LOG_WARNING: "Warning",
	syslog.LOG_NOTICE:  "Notice",
	syslog.LOG_INFO:    "Info",
	syslog.LOG_DEBUG:   "Debug",
}

type Destination interface {
	ForSet(syncSetJson json.RawMessage) error
	ListUsers(desiredAttrs []string) ([]Person, error)
	ApplyChangeSet(changes ChangeSet, activityLog chan<- EventLogItem) ChangeResults
}

type Source interface {
	ForSet(syncSetJson json.RawMessage) error
	ListUsers(desiredAttrs []string) ([]Person, error)
}

type SyncError struct {
	Message   error
	SendAlert bool
}

func (s SyncError) Error() string {
	return s.Message.Error()
}
