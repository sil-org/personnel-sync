package google

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/api/option"

	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
)

// initGoogleAdminService authenticates with the Google API and returns an admin.Service that has the requested scopes
func initGoogleAdminService(auth GoogleAuth, adminEmail string, scopes ...string) (admin.Service, error) {
	googleAuthJson, err := json.Marshal(auth)
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to marshal google auth data into json, error: %s", err.Error())
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, scopes...)
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	ctx := context.TODO()
	config.Subject = adminEmail
	client := config.Client(ctx)

	adminService, err := admin.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return admin.Service{}, fmt.Errorf("unable to retrieve directory Service: %s", err)
	}

	return *adminService, nil
}
