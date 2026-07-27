package google

import (
	"encoding/json"
	"fmt"
	"log/syslog"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/sil-org/personnel-sync/v7/internal"
)

const DefaultSheetName = "Sheet1"

type GoogleSheets struct {
	DestinationConfig internal.DestinationConfig
	SourceConfig      internal.SourceConfig
	GoogleConfig      GoogleConfig
	Service           *sheets.Service
	SheetsSyncSet     SheetsSyncSet
}

type SheetsSyncSet struct {
	SheetID          string
	SheetName        string
	CompareAttribute string
}

func NewGoogleSheetsDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	s, err := readConfig(destinationConfig.ExtraJSON)
	if err != nil {
		return nil, fmt.Errorf("error reading GoogleSheets destination config: %s", err)
	}

	s.DestinationConfig = destinationConfig

	return &s, nil
}

func NewGoogleSheetsSource(sourceConfig internal.SourceConfig) (internal.Source, error) {
	s, err := readConfig(sourceConfig.ExtraJSON)
	if err != nil {
		return nil, fmt.Errorf("error reading GoogleSheets source config: %s", err)
	}
	s.SourceConfig = sourceConfig

	return &s, nil
}

func readConfig(data []byte) (GoogleSheets, error) {
	var s GoogleSheets

	err := json.Unmarshal(data, &s.GoogleConfig)
	if err != nil {
		return s, fmt.Errorf("error unmarshaling GoogleConfig: %s", err)
	}

	s.Service, err = initSheetsService(
		s.GoogleConfig.GoogleAuth,
		s.GoogleConfig.DelegatedAdminEmail,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return s, fmt.Errorf("error initializing Google Sheets service: %s", err)
	}
	return s, nil
}

func initSheetsService(auth GoogleAuth, adminEmail string, scopes ...string) (*sheets.Service, error) {
	googleAuthJson, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config, error: %s", err)
	}

	config.Subject = adminEmail

	ctx := context.Background()
	svc, err := sheets.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service, error: %s", err)
	}
	return svc, nil
}

func (g *GoogleSheets) ForSet(syncSetJson json.RawMessage) error {
	var syncSetConfig SheetsSyncSet
	err := json.Unmarshal(syncSetJson, &syncSetConfig)
	if err != nil {
		return err
	}

	g.SheetsSyncSet = syncSetConfig

	// Defaults
	if g.SheetsSyncSet.SheetName == "" {
		g.SheetsSyncSet.SheetName = DefaultSheetName
	}

	return nil
}

func (g *GoogleSheets) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	if g.DestinationConfig.Type != "" {
		// if this sheet is a destination, don't return the list of users since we don't have logic to do incremental
		// updates to the sheet
		return nil, nil
	}

	sheetData, err := g.readSheet()
	if err != nil {
		return nil, fmt.Errorf("googleSheets ListUsers error %w", err)
	}

	return getPersonsFromSheetData(sheetData, desiredAttrs, g.SheetsSyncSet.CompareAttribute), nil
}

func getPersonsFromSheetData(sheetData [][]any, desiredAttrs []string, compareAttr string) []internal.Person {
	header := map[int]string{}
	if len(sheetData) < 1 {
		return []internal.Person{}
	}

	attrMap := make(map[string]bool, len(desiredAttrs))
	for _, a := range desiredAttrs {
		attrMap[a] = true
	}

	p := make([]internal.Person, len(sheetData)-1)
	for i, row := range sheetData {
		if i == 0 {
			for j, cellValue := range row {
				header[j] = cellValue.(string)
			}
			continue
		}
		p[i-1].Attributes = map[string]string{}
		for j, cellValue := range row {
			if attrMap[header[j]] {
				p[i-1].Attributes[header[j]] = cellValue.(string)
				if header[j] == compareAttr {
					p[i-1].CompareValue = cellValue.(string)
				}
			}
		}
	}
	return p
}

func (g *GoogleSheets) ApplyChangeSet(
	changes internal.ChangeSet,
	eventLog chan<- internal.EventLogItem,
) internal.ChangeResults {
	if g.DestinationConfig.DisableAdd || g.DestinationConfig.DisableDelete || g.DestinationConfig.DisableUpdate {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_INFO,
			Message: fmt.Sprintf("ApplyChangeSet Sync is disabled, no action taken"),
		}
		return internal.ChangeResults{}
	}

	sheetData, err := g.readSheet()
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ALERT,
			Message: fmt.Sprintf("unable to read sheet, error: %v", err),
		}
		return internal.ChangeResults{}
	}

	if err := g.clearSheet(sheetData); err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ALERT,
			Message: fmt.Sprintf("unable to clear sheet, error: %v", err),
		}
		return internal.ChangeResults{}
	}

	if err := g.updateSheet(getHeaderFromSheetData(sheetData), changes.Create); err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ALERT,
			Message: fmt.Sprintf("unable to update sheet, error: %v", err),
		}
		return internal.ChangeResults{}
	}

	return internal.ChangeResults{Created: uint64(len(changes.Create))}
}

func (g *GoogleSheets) readSheet() ([][]any, error) {
	readRange := fmt.Sprintf("%s!A1:ZZ", g.SheetsSyncSet.SheetName)
	resp, err := g.Service.Spreadsheets.Values.Get(g.SheetsSyncSet.SheetID, readRange).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve data from sheet '%s', error: %v", g.SheetsSyncSet.SheetName, err)
	}
	if len(resp.Values) < 1 {
		return nil, fmt.Errorf("no header row found in sheet")
	}
	return resp.Values, nil
}

func getHeaderFromSheetData(sheetData [][]any) map[int]string {
	if len(sheetData) < 1 {
		return map[int]string{}
	}
	header := make(map[int]string, len(sheetData[0]))
	for i, v := range sheetData[0] {
		field := fmt.Sprintf("%v", v)
		header[i] = field
	}
	return header
}

func (g *GoogleSheets) clearSheet(data [][]any) error {
	for i, row := range data {
		if i == 0 {
			continue
		}
		for j := range row {
			data[i][j] = ""
		}
	}
	v := &sheets.ValueRange{
		Values: data,
	}

	updateRange := fmt.Sprintf("%s!A1", g.SheetsSyncSet.SheetName)
	_, err := g.Service.Spreadsheets.Values.
		Update(g.SheetsSyncSet.SheetID, updateRange, v).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to clear sheet, error: %v", err)
	}
	return nil
}

func (g *GoogleSheets) updateSheet(header map[int]string, persons []internal.Person) error {
	table := makeSheetDataFromPersons(header, persons)
	v := &sheets.ValueRange{
		Values: table,
	}

	updateRange := fmt.Sprintf("%s!A2:ZZ", g.SheetsSyncSet.SheetName)
	_, err := g.Service.Spreadsheets.Values.
		Update(g.SheetsSyncSet.SheetID, updateRange, v).
		ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to update sheet, error: %v", err)
	}
	return nil
}

func makeSheetDataFromPersons(header map[int]string, persons []internal.Person) [][]any {
	if len(header) < 1 {
		return [][]any{}
	}
	sheetData := make([][]any, len(persons))
	for i, person := range persons {
		row := make([]any, len(header))
		for j := range row {
			row[j] = person.Attributes[header[j]]
		}
		sheetData[i] = row
	}
	return sheetData
}
