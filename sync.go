package personnel_sync

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sil-org/personnel-sync/v6/alert"
	"github.com/sil-org/personnel-sync/v6/google"
	"github.com/sil-org/personnel-sync/v6/internal"
	"github.com/sil-org/personnel-sync/v6/restapi"
	"github.com/sil-org/personnel-sync/v6/webhelpdesk"
)

func RunSync(configFile string) error {
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
		msg := fmt.Sprintf("Unable to read config, error: %s", err)
		log.Println(msg)
		alert.SendEmail(config.Alert, msg)
		return nil
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
		msg := fmt.Sprintf("Unable to initialize %s source, error: %s", config.Source.Type, err)
		log.Println(msg)
		alert.SendEmail(config.Alert, msg)
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
		msg := fmt.Sprintf("Unable to initialize %s destination, error: %s", config.Destination.Type, err)
		log.Println(msg)
		alert.SendEmail(config.Alert, msg)
		return nil
	}

	maxNameLength := config.MaxSyncSetNameLength()
	var alertList []string

	// Iterate through SyncSets and process changes
	for i, syncSet := range config.SyncSets {
		if syncSet.Disable {
			continue
		}

		if syncSet.Name == "" {
			msg := "configuration contains a set with no name"
			alertList = append(alertList, msg)
		}
		prefix := fmt.Sprintf("[ %-*s ] ", maxNameLength, syncSet.Name)
		syncSetLogger := log.New(os.Stdout, prefix, 0)
		syncSetLogger.Printf("(%v/%v) Beginning sync set", i+1, len(config.SyncSets))

		// Apply SyncSet configs (excluding source/destination as appropriate)
		if err = source.ForSet(syncSet.Source); err != nil {
			err = fmt.Errorf(`Error setting source set on syncSet "%s": %w`, syncSet.Name, err)
			alertList = handleSyncError(syncSetLogger, err, alertList)
		}

		if err = destination.ForSet(syncSet.Destination); err != nil {
			err = fmt.Errorf(`Error setting destination set on syncSet "%s": %w`, syncSet.Name, err)
			alertList = handleSyncError(syncSetLogger, err, alertList)
		}

		if err = internal.RunSyncSet(syncSetLogger, source, destination, config); err != nil {
			err = fmt.Errorf(`Sync failed with error on syncSet "%s": %w`, syncSet.Name, err)
			alertList = handleSyncError(syncSetLogger, err, alertList)
		}
	}

	if len(alertList) > 0 {
		alert.SendEmail(config.Alert, fmt.Sprintf("Sync error(s):\n%s", strings.Join(alertList, "\n")))
	}

	log.Printf("Personnel sync completed at %s", time.Now().UTC().Format(time.RFC1123Z))
	return nil
}

func handleSyncError(logger *log.Logger, err error, alertList []string) []string {
	logger.Println(err)

	var syncError internal.SyncError
	if isSyncError := errors.As(err, &syncError); !isSyncError || syncError.SendAlert {
		alertList = append(alertList, err.Error())
	}
	return alertList
}
