package google

import (
	"encoding/json"
	"fmt"
	"log/syslog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"

	"github.com/sil-org/personnel-sync/v6/internal"

	"golang.org/x/net/context"
)

const (
	RoleMember  = "MEMBER"
	RoleOwner   = "OWNER"
	RoleManager = "MANAGER"
)

type GoogleGroups struct {
	DestinationConfig internal.DestinationConfig
	GoogleConfig      GoogleConfig
	AdminService      admin.Service
	GroupSyncSet      GroupSyncSet
	BatchSize         int
	BatchDelaySeconds int
}

type GroupSyncSet struct {
	GroupEmail    string
	Owners        []string
	ExtraOwners   []string
	Managers      []string
	ExtraManagers []string
	ExtraMembers  []string
	DisableAdd    bool
	DisableUpdate bool
	DisableDelete bool
}

func NewGoogleGroupsDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	var googleGroups GoogleGroups
	// Unmarshal ExtraJSON into GoogleGroupsConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleGroups.GoogleConfig)
	if err != nil {
		return &GoogleGroups{}, err
	}

	// Defaults
	if googleGroups.BatchSize <= 0 {
		googleGroups.BatchSize = DefaultBatchSize
	}
	if googleGroups.BatchDelaySeconds <= 0 {
		googleGroups.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	// Initialize AdminService object
	googleGroups.AdminService, err = initGoogleAdminService(
		googleGroups.GoogleConfig.GoogleAuth,
		googleGroups.GoogleConfig.DelegatedAdminEmail,
		admin.AdminDirectoryGroupScope,
		admin.AdminDirectoryGroupMemberScope,
	)
	if err != nil {
		return &GoogleGroups{}, err
	}

	return &googleGroups, nil
}

func (g *GoogleGroups) ForSet(syncSetJson json.RawMessage) error {
	var syncSetConfig GroupSyncSet
	err := json.Unmarshal(syncSetJson, &syncSetConfig)
	if err != nil {
		return err
	}

	if syncSetConfig.GroupEmail == "" {
		return fmt.Errorf("GroupEmail missing from sync set json")
	}

	g.GroupSyncSet = syncSetConfig

	return nil
}

func (g *GoogleGroups) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	var membersList []*admin.Member
	membersListCall := g.AdminService.Members.List(g.GroupSyncSet.GroupEmail)
	err := membersListCall.Pages(context.TODO(), func(members *admin.Members) error {
		membersList = append(membersList, members.Members...)
		return nil
	})
	if err != nil {
		syncErr := internal.SyncError{
			Message:   fmt.Errorf("unable to get members of group %s: %w", g.GroupSyncSet.GroupEmail, err),
			SendAlert: true,
		}
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusServiceUnavailable {
			syncErr.SendAlert = false
		}
		return []internal.Person{}, syncErr
	}

	var members []internal.Person

	for _, nextMember := range membersList {
		// Do not include ExtraManager, ExtraOwners, or ExtraMember in list to prevent inclusion in delete list
		if slices.Contains(g.GroupSyncSet.ExtraManagers, nextMember.Email) ||
			slices.Contains(g.GroupSyncSet.ExtraOwners, nextMember.Email) ||
			slices.Contains(g.GroupSyncSet.ExtraMembers, nextMember.Email) {
			continue
		}

		members = append(members, internal.Person{
			CompareValue: nextMember.Email,
			Attributes: map[string]string{
				"Email": strings.ToLower(nextMember.Email),
			},
		})
	}

	return members, nil
}

func (g *GoogleGroups) ApplyChangeSet(
	changes internal.ChangeSet,
	eventLog chan<- internal.EventLogItem,
) internal.ChangeResults {
	var results internal.ChangeResults
	var wg sync.WaitGroup

	// key = email, value = role
	toBeCreated := map[string]string{}
	for _, person := range changes.Create {
		toBeCreated[person.CompareValue] = RoleMember
	}

	// Update Owner / Manager roles
	for _, owner := range g.GroupSyncSet.Owners {
		if _, ok := toBeCreated[owner]; ok {
			toBeCreated[owner] = RoleOwner
		}
	}
	for _, manager := range g.GroupSyncSet.Managers {
		if _, ok := toBeCreated[manager]; ok {
			toBeCreated[manager] = RoleManager
		}
	}

	// Add any ExtraManagers, ExtraOwners, and ExtraMembers to Create list since they are not in the source people
	for _, manager := range g.GroupSyncSet.ExtraManagers {
		toBeCreated[manager] = RoleManager
	}
	for _, owner := range g.GroupSyncSet.ExtraOwners {
		toBeCreated[owner] = RoleOwner
	}
	for _, member := range g.GroupSyncSet.ExtraMembers {
		toBeCreated[member] = RoleMember
	}

	// One minute per batch
	batchTimer := internal.NewBatchTimer(g.BatchSize, g.BatchDelaySeconds)

	if !g.GroupSyncSet.DisableAdd {
		for email, role := range toBeCreated {
			wg.Add(1)
			go g.addMember(email, role, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if !g.GroupSyncSet.DisableDelete {
		for _, dp := range changes.Delete {
			// Do not delete ExtraManagers, ExtraOwners, or ExtraMembers
			if slices.Contains(g.GroupSyncSet.ExtraManagers, dp.CompareValue) ||
				slices.Contains(g.GroupSyncSet.ExtraOwners, dp.CompareValue) ||
				slices.Contains(g.GroupSyncSet.ExtraMembers, dp.CompareValue) {
				continue
			}
			wg.Add(1)
			go g.removeMember(dp.CompareValue, &results.Deleted, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func (g *GoogleGroups) addMember(
	email, role string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	newMember := admin.Member{
		Role:  role,
		Email: email,
	}

	_, err := g.AdminService.Members.Insert(g.GroupSyncSet.GroupEmail, &newMember).Do()
	if err != nil && !strings.Contains(err.Error(), "409") { // error code 409 is for existing user
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to insert %s in Google group %s: %s", email, g.GroupSyncSet.GroupEmail, err.Error()),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "AddMember " + email,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleGroups) removeMember(
	email string,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	err := g.AdminService.Members.Delete(g.GroupSyncSet.GroupEmail, email).Do()
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to delete %s from Google group %s: %s", email, g.GroupSyncSet.GroupEmail, err.Error()),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "RemoveMember " + email,
	}

	atomic.AddUint64(counter, 1)
}
