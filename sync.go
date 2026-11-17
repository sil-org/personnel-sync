package personnel_sync

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sil-org/personnel-sync/v8/alert"
	"github.com/sil-org/personnel-sync/v8/google"
	"github.com/sil-org/personnel-sync/v8/internal"
	"github.com/sil-org/personnel-sync/v8/restapi"
	"github.com/sil-org/personnel-sync/v8/webhelpdesk"

	"github.com/getsentry/sentry-go"
)

// maxReportingMargin is the most time RunSync reserves before ctx's deadline for reporting (logging and alerts).
const maxReportingMargin = 10 * time.Second

// errReportingMargin is the cancellation cause when RunSync stops early to leave time for reporting.
var errReportingMargin = errors.New("stopped early to leave time for reporting before the deadline")

// RunSync loads the config file and runs every enabled sync set. The ctx bounds all outbound requests. If ctx has a
// deadline (such as a Lambda timeout), RunSync stops sync work shortly before it, so that it can still log and send
// alerts about the interrupted sync. Alerts themselves are not bound by ctx.
func RunSync(ctx context.Context, configFile string) error {
	ctx, cancel := withReportingMargin(ctx)
	defer cancel()

	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	log.Printf("Personnel sync started at %s", time.Now().UTC().Format(time.RFC1123Z))

	rawConfig, err := internal.LoadConfig(configFile)
	if err != nil {
		msg := fmt.Sprintf("Unable to load config, error: %s", err)
		log.Println(msg)
		return nil
	}

	config, err := internal.ReadConfig(rawConfig)
	if err != nil {
		msg := fmt.Errorf("Unable to read config, error: %w", err)
		log.Println(msg)
		alert.New(config.Alert, msg)
		return nil
	}

	if config.Runtime.SentryDSN != "" {
		initSentry(config.Runtime)
	}

	// Instantiate Source
	var source internal.Source
	switch config.Source.Type {
	case internal.SourceTypeRestAPI:
		source, err = restapi.NewRestAPISource(config.Source)
	case internal.SourceTypeGoogleSheets:
		source, err = google.NewGoogleSheetsSource(config.Source)
	default:
		err = errors.New("unrecognized source type")
	}

	if err != nil {
		msg := fmt.Errorf("Unable to initialize %s source, error: %w", config.Source.Type, err)
		log.Println(msg.Error())
		alert.New(config.Alert, msg)
		return nil
	}

	// Instantiate Destination
	var destination internal.Destination
	switch config.Destination.Type {
	case internal.DestinationTypeGoogleContacts:
		destination, err = google.NewGoogleContactsDestination(config.Destination)
	case internal.DestinationTypeGoogleGroups:
		destination, err = google.NewGoogleGroupsDestination(config.Destination)
	case internal.DestinationTypeGoogleSheets:
		destination, err = google.NewGoogleSheetsDestination(config.Destination)
	case internal.DestinationTypeGoogleUsers:
		destination, err = google.NewGoogleUsersDestination(config.Destination)
	case internal.DestinationTypeRestAPI:
		destination, err = restapi.NewRestAPIDestination(config.Destination)
	case internal.DestinationTypeWebHelpDesk:
		destination, err = webhelpdesk.NewWebHelpDeskDestination(config.Destination)
	default:
		err = errors.New("unrecognized destination type")
	}

	if err != nil {
		msg := fmt.Errorf("Unable to initialize %s destination, error: %w", config.Destination.Type, err)
		log.Println(msg)
		alert.New(config.Alert, msg)
		return nil
	}

	maxNameLength := config.MaxSyncSetNameLength()
	var alertError error

	// Iterate through SyncSets and process changes
	for i, syncSet := range config.SyncSets {
		if syncSet.Disable {
			continue
		}

		if ctx.Err() != nil {
			err := fmt.Errorf(`Sync stopped before syncSet "%s" (%d of %d): %w`,
				syncSet.Name, i+1, len(config.SyncSets), context.Cause(ctx))
			log.Println(err)
			alertError = errors.Join(alertError, err)
			break
		}

		if syncSet.Name == "" {
			err := errors.New("configuration contains a set with no name")
			alertError = errors.Join(alertError, err)
		}
		prefix := fmt.Sprintf("[ %-*s ] ", maxNameLength, syncSet.Name)
		syncSetLogger := log.New(os.Stdout, prefix, 0)
		syncSetLogger.Printf("(%v/%v) Beginning sync set", i+1, len(config.SyncSets))

		// Apply SyncSet configs (excluding source/destination as appropriate)
		if err = source.ForSet(syncSet.Source); err != nil {
			err = fmt.Errorf(`Error setting source set on syncSet "%s": %w`, syncSet.Name, err)
			alertError = handleSyncError(syncSetLogger, err, alertError)
		}

		if err = destination.ForSet(syncSet.Destination); err != nil {
			err = fmt.Errorf(`Error setting destination set on syncSet "%s": %w`, syncSet.Name, err)
			alertError = handleSyncError(syncSetLogger, err, alertError)
		}

		if err = internal.RunSyncSet(ctx, syncSetLogger, source, destination, config); err != nil {
			err = fmt.Errorf(`Sync failed with error on syncSet "%s": %w`, syncSet.Name, err)
			alertError = handleSyncError(syncSetLogger, err, alertError)
		}
	}

	if alertError != nil {
		alertError = errors.Join(errors.New("Sync error(s):"), alertError)
		alert.New(config.Alert, alertError)
	}

	log.Printf("Personnel sync completed at %s", time.Now().UTC().Format(time.RFC1123Z))
	return nil
}

// withReportingMargin returns a context whose deadline is a little earlier than ctx's, leaving time to report on an
// interrupted sync. The margin is maxReportingMargin, or a tenth of the remaining time if that is shorter. If ctx has
// no deadline, the returned context has none either.
func withReportingMargin(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return context.WithCancel(ctx)
	}
	margin := min(maxReportingMargin, time.Until(deadline)/10)
	return context.WithDeadlineCause(ctx, deadline.Add(-margin), errReportingMargin)
}

func handleSyncError(logger *log.Logger, err, alert error) error {
	logger.Println(err)

	var syncError internal.SyncError
	if isSyncError := errors.As(err, &syncError); !isSyncError || syncError.SendAlert {
		alert = errors.Join(alert, err)
	}
	return alert
}

func initSentry(config internal.RuntimeConfig) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         config.SentryDSN,
		Environment: config.Environment,
		Tags:        config.SentryTags,
	})
	if err != nil {
		log.Printf("sentry.Init failure: %s", err)
	}
}
